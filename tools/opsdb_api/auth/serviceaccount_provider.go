//# tools/opsdb-api/auth/serviceaccount_provider.go

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ServiceAccountProvider implements auth.Provider for runner service accounts.
// Validates tokens issued during runner registration, resolves to runner_machine
// and runner_spec identities.
type ServiceAccountProvider struct {
	validationMethod string // hmac, jwt, vault_lookup
	hmacSecret       []byte
	vaultAddr        string
	vaultTokenPath   string
	httpClient       httpDoer

	// runnerMapping caches service account name → runner identity.
	runnerMapping     map[string]*runnerIdentity
	runnerMappingLock sync.RWMutex

	db RunnerLookup
}

// httpDoer is a minimal interface for HTTP requests, satisfied by http.Client.
type httpDoer interface {
	Do(req *httpRequest) (*httpResponse, error)
}

// RunnerLookup is the interface for resolving service accounts to runner rows.
type RunnerLookup interface {
	QueryRow(query string, args ...interface{}) RowScanner
}

// runnerIdentity holds the cached mapping from a service account to its
// runner_machine and runner_spec IDs.
type runnerIdentity struct {
	RunnerMachineID int
	RunnerSpecID    int
	AccountName     string
}

// ServiceAccountConfig holds service account provider configuration.
type ServiceAccountConfig struct {
	ValidationMethod string `yaml:"validation_method"` // hmac, jwt, vault_lookup
	HMACSecretPath   string `yaml:"hmac_secret_path"`
	VaultAddr        string `yaml:"vault_addr"`
	VaultTokenPath   string `yaml:"vault_token_path"`
}

// serviceAccountClaims represents the claims in a service account token.
type serviceAccountClaims struct {
	Subject     string `json:"sub"`
	Issuer      string `json:"iss"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
	AccountName string `json:"account_name"`
	MachineID   int    `json:"machine_id,omitempty"`
	SpecID      int    `json:"spec_id,omitempty"`
	// OnBehalfOf carries human identity for web-mediated operations
	OnBehalfOf string `json:"on_behalf_of,omitempty"`
	OnBehalfID int    `json:"on_behalf_id,omitempty"`
}

// parseServiceAccountConfigFile reads service account configuration from YAML.
func parseServiceAccountConfigFile(path string) (*ServiceAccountConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account config %s: %w", path, err)
	}
	cfg := &ServiceAccountConfig{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account config %s: %w", path, err)
	}

	if cfg.ValidationMethod == "" {
		cfg.ValidationMethod = "hmac"
	}

	switch cfg.ValidationMethod {
	case "hmac":
		if cfg.HMACSecretPath == "" {
			return nil, fmt.Errorf("service account config: hmac validation requires hmac_secret_path")
		}
	case "vault_lookup":
		if cfg.VaultAddr == "" {
			return nil, fmt.Errorf("service account config: vault_lookup validation requires vault_addr")
		}
	case "jwt":
		// JWT validation uses the same mechanism as OIDC but with
		// a local signing key; config path points to the public key
	default:
		return nil, fmt.Errorf("unknown service account validation method: %s (supported: hmac, jwt, vault_lookup)", cfg.ValidationMethod)
	}

	return cfg, nil
}

// newServiceAccountProviderFromConfig creates a service account provider
// from parsed configuration.
func newServiceAccountProviderFromConfig(cfg *ServiceAccountConfig) (Provider, error) {
	provider := &ServiceAccountProvider{
		validationMethod: cfg.ValidationMethod,
		runnerMapping:    make(map[string]*runnerIdentity),
	}

	switch cfg.ValidationMethod {
	case "hmac":
		secret, err := os.ReadFile(cfg.HMACSecretPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read HMAC secret from %s: %w", cfg.HMACSecretPath, err)
		}
		provider.hmacSecret = secret

	case "vault_lookup":
		provider.vaultAddr = cfg.VaultAddr
		provider.vaultTokenPath = cfg.VaultTokenPath
	}

	return provider, nil
}

// Authenticate validates a service account token and resolves the runner identity.
func (p *ServiceAccountProvider) Authenticate(creds Credentials) (*Identity, error) {
	if creds.BearerToken == "" {
		return nil, fmt.Errorf("no bearer token provided for service account authentication")
	}

	claims, err := p.validateToken(creds.BearerToken)
	if err != nil {
		return nil, fmt.Errorf("service account token validation failed: %w", err)
	}

	// resolve runner identity from claims or database
	runnerID, err := p.resolveRunner(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runner identity for %s: %w", claims.AccountName, err)
	}

	identity := &Identity{
		RunnerMachineID: &runnerID.RunnerMachineID,
		RunnerSpecID:    &runnerID.RunnerSpecID,
		Username:        claims.AccountName,
		AuthMethod:      "service_account",
	}

	// handle web-mediated operations where a runner carries a human identity
	if claims.OnBehalfOf != "" && claims.OnBehalfID > 0 {
		identity.OpsUserID = &claims.OnBehalfID
		identity.IsWebMediated = true
	}

	return identity, nil
}

// RefreshToken requests a new service account token. For HMAC tokens,
// this generates a new token with a fresh expiration. For vault-based
// tokens, this renews the vault token.
func (p *ServiceAccountProvider) RefreshToken(token string) (*Identity, error) {
	// validate the current token first to extract claims
	claims, err := p.validateToken(token)
	if err != nil {
		return nil, fmt.Errorf("cannot refresh invalid token: %w", err)
	}

	switch p.validationMethod {
	case "vault_lookup":
		err := p.renewVaultToken(token)
		if err != nil {
			return nil, fmt.Errorf("vault token renewal failed: %w", err)
		}
		// re-authenticate with the renewed token
		return p.Authenticate(Credentials{BearerToken: token})

	default:
		return nil, fmt.Errorf("token refresh not supported for %s validation; "+
			"request a new token from the secret backend for account %s",
			p.validationMethod, claims.AccountName)
	}
}

// Type returns "service_account".
func (p *ServiceAccountProvider) Type() string {
	return "service_account"
}

// SetDB sets the database handle for runner_machine lookups.
func (p *ServiceAccountProvider) SetDB(db RunnerLookup) {
	p.db = db
}

// validateToken validates a service account token using the configured method.
func (p *ServiceAccountProvider) validateToken(tokenStr string) (*serviceAccountClaims, error) {
	switch p.validationMethod {
	case "hmac":
		return p.validateHMACToken(tokenStr)
	case "vault_lookup":
		return p.validateVaultToken(tokenStr)
	case "jwt":
		return p.validateJWTToken(tokenStr)
	default:
		return nil, fmt.Errorf("unknown validation method: %s", p.validationMethod)
	}
}

// validateHMACToken validates a token signed with HMAC-SHA256.
// Token format: base64url(json_claims).base64url(hmac_signature)
func (p *ServiceAccountProvider) validateHMACToken(tokenStr string) (*serviceAccountClaims, error) {
	parts := strings.SplitN(tokenStr, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("malformed HMAC token: expected two dot-separated parts")
	}

	claimsB64 := parts[0]
	sigB64 := parts[1]

	// verify signature
	mac := hmac.New(sha256.New, p.hmacSecret)
	mac.Write([]byte(claimsB64))
	expectedSig := mac.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token signature: %w", err)
	}

	if !hmac.Equal(actualSig, expectedSig) {
		return nil, fmt.Errorf("token signature verification failed")
	}

	// decode and parse claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(claimsB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode token claims: %w", err)
	}

	claims := &serviceAccountClaims{}
	err = json.Unmarshal(claimsJSON, claims)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token claims: %w", err)
	}

	// validate expiration
	now := time.Now().Unix()
	if claims.ExpiresAt > 0 && now > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired at %d (now %d)", claims.ExpiresAt, now)
	}

	// validate not issued in the future
	if claims.IssuedAt > now+60 {
		return nil, fmt.Errorf("token issued in the future: iat=%d now=%d", claims.IssuedAt, now)
	}

	if claims.AccountName == "" && claims.Subject == "" {
		return nil, fmt.Errorf("token missing both account_name and subject claims")
	}
	if claims.AccountName == "" {
		claims.AccountName = claims.Subject
	}

	return claims, nil
}

// validateVaultToken validates a token by looking it up against the
// Vault token lookup endpoint.
func (p *ServiceAccountProvider) validateVaultToken(tokenStr string) (*serviceAccountClaims, error) {
	lookupURL := strings.TrimRight(p.vaultAddr, "/") + "/v1/auth/token/lookup-self"

	req, err := newHTTPRequest("GET", lookupURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault lookup request: %w", err)
	}
	req.Header.Set("X-Vault-Token", tokenStr)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vault token lookup failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("vault token lookup returned status %d", resp.StatusCode)
	}

	body, err := readAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault lookup response: %w", err)
	}

	var result struct {
		Data struct {
			DisplayName string                 `json:"display_name"`
			Metadata    map[string]interface{} `json:"meta"`
			TTL         int                    `json:"ttl"`
			ExpireTime  string                 `json:"expire_time"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse vault lookup response: %w", err)
	}

	if result.Data.TTL <= 0 {
		return nil, fmt.Errorf("vault token is expired or has no TTL")
	}

	accountName := result.Data.DisplayName
	if accountName == "" {
		if name, ok := result.Data.Metadata["account_name"].(string); ok {
			accountName = name
		}
	}
	if accountName == "" {
		return nil, fmt.Errorf("vault token has no display_name or account_name metadata")
	}

	claims := &serviceAccountClaims{
		AccountName: accountName,
	}

	// extract machine_id and spec_id from metadata if present
	if mid, ok := result.Data.Metadata["machine_id"]; ok {
		if midFloat, ok := mid.(float64); ok {
			claims.MachineID = int(midFloat)
		}
	}
	if sid, ok := result.Data.Metadata["spec_id"]; ok {
		if sidFloat, ok := sid.(float64); ok {
			claims.SpecID = int(sidFloat)
		}
	}

	return claims, nil
}

// validateJWTToken validates a service account JWT token using a local
// signing key. This is a simplified JWT validation path for environments
// that issue their own JWTs for service accounts.
func (p *ServiceAccountProvider) validateJWTToken(tokenStr string) (*serviceAccountClaims, error) {
	// JWT validation for service accounts follows the same pattern
	// as OIDC but with a locally configured key rather than JWKS discovery.
	// For now, fall back to HMAC validation as most deployments will use
	// either HMAC or vault_lookup.
	return nil, fmt.Errorf("JWT service account validation not yet implemented; use hmac or vault_lookup")
}

// resolveRunner maps a service account to its runner_machine and runner_spec IDs.
// If the token claims include machine_id and spec_id, uses those directly.
// Otherwise looks up via database.
func (p *ServiceAccountProvider) resolveRunner(claims *serviceAccountClaims) (*runnerIdentity, error) {
	// if claims carry IDs directly, use them
	if claims.MachineID > 0 && claims.SpecID > 0 {
		return &runnerIdentity{
			RunnerMachineID: claims.MachineID,
			RunnerSpecID:    claims.SpecID,
			AccountName:     claims.AccountName,
		}, nil
	}

	// check cache
	p.runnerMappingLock.RLock()
	if cached, ok := p.runnerMapping[claims.AccountName]; ok {
		p.runnerMappingLock.RUnlock()
		return cached, nil
	}
	p.runnerMappingLock.RUnlock()

	if p.db == nil {
		return nil, fmt.Errorf("no database configured for runner lookup and token claims missing machine_id/spec_id")
	}

	// look up runner_machine by service account name
	var machineID, specID int
	err := p.db.QueryRow(
		"SELECT rm.id, rm.runner_spec_id FROM runner_machine rm "+
			"WHERE rm.service_account_name = $1 AND rm.is_active = true LIMIT 1",
		claims.AccountName,
	).Scan(&machineID, &specID)
	if err != nil {
		return nil, fmt.Errorf("runner_machine not found for service account %s: %w", claims.AccountName, err)
	}

	identity := &runnerIdentity{
		RunnerMachineID: machineID,
		RunnerSpecID:    specID,
		AccountName:     claims.AccountName,
	}

	// cache the mapping
	p.runnerMappingLock.Lock()
	p.runnerMapping[claims.AccountName] = identity
	p.runnerMappingLock.Unlock()

	return identity, nil
}

// renewVaultToken renews a Vault token via the renew-self endpoint.
func (p *ServiceAccountProvider) renewVaultToken(tokenStr string) error {
	renewURL := strings.TrimRight(p.vaultAddr, "/") + "/v1/auth/token/renew-self"

	req, err := newHTTPRequest("POST", renewURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create vault renew request: %w", err)
	}
	req.Header.Set("X-Vault-Token", tokenStr)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("vault token renewal failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("vault token renewal returned status %d", resp.StatusCode)
	}

	return nil
}

// Thin wrappers over net/http to keep the httpDoer interface minimal
// and avoid importing net/http types in the interface definition.

type httpRequest struct {
	Method string
	URL    string
	Header map[string]string
	Body   []byte
}

type httpResponse struct {
	StatusCode int
	Body       readCloser
}

type readCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}

func newHTTPRequest(method string, url string, body []byte) (*httpRequest, error) {
	return &httpRequest{
		Method: method,
		URL:    url,
		Header: make(map[string]string),
		Body:   body,
	}, nil
}

func readAll(r readCloser) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}
