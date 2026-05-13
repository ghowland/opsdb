//# tools/importers/opsdb-import-secrets/vault.go

package secrets

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Observation is the common observation structure for secret metadata importers.
type Observation struct {
	SecretPath         string
	Engine             string
	Version            string
	LastRotatedTime    string
	RotationEnabled    bool
	ExpirationTime     string
	RotationCompliance string
	Tags               map[string]string
}

// VaultConfig holds configuration for the Vault metadata importer.
type VaultConfig struct {
	Address    string
	TokenPath  string
	MountPaths []string // which secret engine mounts to scan; empty = discover all
	MaxDepth   int
}

// vaultClient wraps HTTP calls to the Vault API.
type vaultClient struct {
	address    string
	token      string
	httpClient *http.Client
}

// vaultSecretMetadata holds metadata for one secret from the Vault metadata endpoint.
type vaultSecretMetadata struct {
	Path           string
	Mount          string
	CurrentVersion int
	CreatedTime    *time.Time
	UpdatedTime    *time.Time
	OldestVersion  int
	MaxVersions    int
	CustomMetadata map[string]string
}

// ImportVault reads secret paths and metadata from HashiCorp Vault.
// NEVER reads secret values. Only metadata: paths, versions, creation times,
// rotation timestamps, expiration dates.
func ImportVault(config *VaultConfig) ([]Observation, error) {
	if config.Address == "" {
		return nil, fmt.Errorf("vault address is required")
	}

	token, err := readVaultToken(config.TokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault token: %w", err)
	}

	client := &vaultClient{
		address:    strings.TrimRight(config.Address, "/"),
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	maxDepth := config.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 10
	}

	// discover or use configured mount paths
	mounts := config.MountPaths
	if len(mounts) == 0 {
		discovered, err := client.listSecretMounts()
		if err != nil {
			return nil, fmt.Errorf("failed to discover secret mounts: %w", err)
		}
		mounts = discovered
	}

	var allObservations []Observation
	now := time.Now().UTC()

	for _, mount := range mounts {
		mount = strings.TrimRight(mount, "/")

		paths, err := client.listSecretsRecursive(mount, "", maxDepth, 0)
		if err != nil {
			return allObservations, fmt.Errorf("failed to list secrets in mount %s: %w", mount, err)
		}

		for _, path := range paths {
			metadata, err := client.readSecretMetadata(mount, path)
			if err != nil {
				continue
			}

			obs := mapVaultMetadataToObservation(metadata, now)
			allObservations = append(allObservations, obs)
		}
	}

	return allObservations, nil
}

// listSecretMounts discovers KV secret engine mounts from the Vault sys/mounts endpoint.
func (c *vaultClient) listSecretMounts() ([]string, error) {
	resp, err := c.vaultGet("/v1/sys/mounts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sys/mounts returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read mounts response: %w", err)
	}

	var result struct {
		Data map[string]struct {
			Type    string `json:"type"`
			Options struct {
				Version string `json:"version"`
			} `json:"options"`
		} `json:"data"`
	}

	var topLevel map[string]struct {
		Type    string `json:"type"`
		Options struct {
			Version string `json:"version"`
		} `json:"options"`
	}

	if err := json.Unmarshal(body, &result); err == nil && result.Data != nil {
		topLevel = result.Data
	} else {
		json.Unmarshal(body, &topLevel)
	}

	var mounts []string
	for path, mount := range topLevel {
		if mount.Type == "kv" || mount.Type == "generic" {
			mounts = append(mounts, strings.TrimRight(path, "/"))
		}
	}

	return mounts, nil
}

// listSecretsRecursive lists secret paths under a mount recursively using
// LIST operations. Never reads secret data.
func (c *vaultClient) listSecretsRecursive(mount string, prefix string, maxDepth int, currentDepth int) ([]string, error) {
	if currentDepth >= maxDepth {
		return nil, nil
	}

	listPath := fmt.Sprintf("/v1/%s/metadata/%s", mount, prefix)
	resp, err := c.vaultList(listPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LIST %s returned status %d", listPath, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read list response: %w", err)
	}

	var result struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}

	var allPaths []string

	for _, key := range result.Data.Keys {
		fullPath := prefix + key

		if strings.HasSuffix(key, "/") {
			children, err := c.listSecretsRecursive(mount, fullPath, maxDepth, currentDepth+1)
			if err != nil {
				continue
			}
			allPaths = append(allPaths, children...)
		} else {
			allPaths = append(allPaths, fullPath)
		}
	}

	return allPaths, nil
}

// readSecretMetadata reads metadata for a specific secret path via the
// KV v2 metadata endpoint. NEVER reads the data endpoint.
func (c *vaultClient) readSecretMetadata(mount string, path string) (*vaultSecretMetadata, error) {
	metadataPath := fmt.Sprintf("/v1/%s/metadata/%s", mount, path)

	resp, err := c.vaultGet(metadataPath)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata read for %s returned status %d", path, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata response: %w", err)
	}

	var result struct {
		Data struct {
			CurrentVersion int               `json:"current_version"`
			OldestVersion  int               `json:"oldest_version"`
			MaxVersions    int               `json:"max_versions"`
			CreatedTime    string            `json:"created_time"`
			UpdatedTime    string            `json:"updated_time"`
			CustomMetadata map[string]string `json:"custom_metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse metadata for %s: %w", path, err)
	}

	metadata := &vaultSecretMetadata{
		Path:           path,
		Mount:          mount,
		CurrentVersion: result.Data.CurrentVersion,
		OldestVersion:  result.Data.OldestVersion,
		MaxVersions:    result.Data.MaxVersions,
		CustomMetadata: result.Data.CustomMetadata,
	}

	if result.Data.CreatedTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, result.Data.CreatedTime); err == nil {
			metadata.CreatedTime = &t
		}
	}
	if result.Data.UpdatedTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, result.Data.UpdatedTime); err == nil {
			metadata.UpdatedTime = &t
		}
	}

	return metadata, nil
}

// mapVaultMetadataToObservation converts Vault secret metadata into an Observation.
func mapVaultMetadataToObservation(metadata *vaultSecretMetadata, now time.Time) Observation {
	obs := Observation{
		SecretPath: fmt.Sprintf("%s/%s", metadata.Mount, metadata.Path),
		Engine:     "vault_kv",
		Version:    fmt.Sprintf("%d", metadata.CurrentVersion),
		Tags:       make(map[string]string),
	}

	for k, v := range metadata.CustomMetadata {
		obs.Tags[k] = v
	}
	obs.Tags["vault_mount"] = metadata.Mount

	if metadata.UpdatedTime != nil {
		obs.LastRotatedTime = metadata.UpdatedTime.Format(time.RFC3339)
	}

	if rotationEnabled, ok := metadata.CustomMetadata["rotation_enabled"]; ok {
		obs.RotationEnabled = rotationEnabled == "true"
	}

	if expirationStr, ok := metadata.CustomMetadata["expiration_time"]; ok {
		obs.ExpirationTime = expirationStr
	}

	obs.RotationCompliance = computeVaultRotationCompliance(metadata, now)

	return obs
}

// computeVaultRotationCompliance determines compliance status based on
// custom metadata rotation policy and last update time.
func computeVaultRotationCompliance(metadata *vaultSecretMetadata, now time.Time) string {
	rotationDaysStr, hasRotationPolicy := metadata.CustomMetadata["rotation_interval_days"]
	if !hasRotationPolicy {
		return "no_rotation_policy"
	}

	var rotationDays int
	_, err := fmt.Sscanf(rotationDaysStr, "%d", &rotationDays)
	if err != nil || rotationDays <= 0 {
		return "invalid_rotation_policy"
	}

	if metadata.UpdatedTime == nil {
		return "never_updated"
	}

	daysSinceUpdate := int(now.Sub(*metadata.UpdatedTime).Hours() / 24)

	if daysSinceUpdate > rotationDays {
		return fmt.Sprintf("overdue_by_%d_days", daysSinceUpdate-rotationDays)
	}

	if daysSinceUpdate > rotationDays-7 {
		return "rotation_due_soon"
	}

	return "compliant"
}

// vaultGet performs an authenticated GET request to the Vault API.
func (c *vaultClient) vaultGet(path string) (*http.Response, error) {
	url := c.address + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", path, err)
	}
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", path, err)
	}

	return resp, nil
}

// vaultList performs an authenticated LIST request to the Vault API.
func (c *vaultClient) vaultList(path string) (*http.Response, error) {
	url := c.address + path + "?list=true"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list request for %s: %w", path, err)
	}
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list request to %s failed: %w", path, err)
	}

	return resp, nil
}

// readVaultToken reads the Vault token from a file path.
func readVaultToken(tokenPath string) (string, error) {
	if tokenPath == "" {
		return "", fmt.Errorf("vault token path is empty")
	}

	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read token from %s: %w", tokenPath, err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("vault token file %s is empty", tokenPath)
	}

	return token, nil
}
