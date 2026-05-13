// === importers/opsdb_import_identity/azuread.go ===
package identity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// ImportAzureADUsers reads users from Microsoft Graph API and maps to ops_user observations.
func ImportAzureADUsers(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client, err := newAzureADClient(config)
	if err != nil {
		return results, fmt.Errorf("azure ad client init: %w", err)
	}

	nextURL := client.baseURL + "/v1.0/users?$top=" + fmt.Sprintf("%d", config.BatchSize) +
		"&$select=id,userPrincipalName,displayName,mail,givenName,surname,jobTitle," +
		"department,companyName,accountEnabled,userType,createdDateTime," +
		"lastSignInDateTime,mobilePhone,officeLocation"

	for nextURL != "" {
		body, next, err := client.graphGet(nextURL)
		if err != nil {
			return results, fmt.Errorf("azure ad list users: %w", err)
		}

		items, ok := body["value"].([]interface{})
		if !ok {
			break
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			upn := azGetString(m, "userPrincipalName")
			if upn == "" {
				continue
			}

			displayName := azGetString(m, "displayName")
			accountEnabled := azGetBool(m, "accountEnabled")
			userType := azGetString(m, "userType")

			obs := Observation{
				EntityType: "ops_user",
				EntityID:   upn,
				StateKey:   "azuread_user",
				Value:      displayName,
				DataJSON: map[string]interface{}{
					"azure_id":            azGetString(m, "id"),
					"user_principal_name": upn,
					"display_name":        displayName,
					"email":               azGetString(m, "mail"),
					"given_name":          azGetString(m, "givenName"),
					"surname":             azGetString(m, "surname"),
					"title":               azGetString(m, "jobTitle"),
					"department":          azGetString(m, "department"),
					"company_name":        azGetString(m, "companyName"),
					"mobile_phone":        azGetString(m, "mobilePhone"),
					"office_location":     azGetString(m, "officeLocation"),
					"is_active":           accountEnabled,
					"user_type":           userType,
					"is_guest":            userType == "Guest",
					"created_time":        azGetString(m, "createdDateTime"),
					"last_sign_in_time":   azGetString(m, "lastSignInDateTime"),
					"source":              "azuread",
					"tenant_id":           config.TenantID,
				},
			}
			results = append(results, obs)
		}

		nextURL = next
	}

	return results, nil
}

// ImportAzureADGroups reads groups from Microsoft Graph API and maps to ops_group observations.
func ImportAzureADGroups(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client, err := newAzureADClient(config)
	if err != nil {
		return results, fmt.Errorf("azure ad client init: %w", err)
	}

	nextURL := client.baseURL + "/v1.0/groups?$top=" + fmt.Sprintf("%d", config.BatchSize) +
		"&$select=id,displayName,description,groupTypes,securityEnabled,mailEnabled," +
		"mail,createdDateTime,membershipRule,membershipRuleProcessingState"

	for nextURL != "" {
		body, next, err := client.graphGet(nextURL)
		if err != nil {
			return results, fmt.Errorf("azure ad list groups: %w", err)
		}

		items, ok := body["value"].([]interface{})
		if !ok {
			break
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			name := azGetString(m, "displayName")
			if name == "" {
				continue
			}

			if isExcludedGroup(name, config.ExcludeGroups) {
				continue
			}

			groupTypes := azGetStringList(m, "groupTypes")
			groupClassification := classifyAzureGroup(m, groupTypes)

			obs := Observation{
				EntityType: "ops_group",
				EntityID:   name,
				StateKey:   "azuread_group",
				Value:      name,
				DataJSON: map[string]interface{}{
					"azure_id":              azGetString(m, "id"),
					"name":                  name,
					"description":           azGetString(m, "description"),
					"group_types":           groupTypes,
					"group_classification":  groupClassification,
					"security_enabled":      azGetBool(m, "securityEnabled"),
					"mail_enabled":          azGetBool(m, "mailEnabled"),
					"mail":                  azGetString(m, "mail"),
					"membership_rule":       azGetString(m, "membershipRule"),
					"membership_processing": azGetString(m, "membershipRuleProcessingState"),
					"created_time":          azGetString(m, "createdDateTime"),
					"source":                "azuread",
					"tenant_id":             config.TenantID,
				},
			}
			results = append(results, obs)
		}

		nextURL = next
	}

	return results, nil
}

// ImportAzureADGroupMemberships reads group memberships from Microsoft Graph API
// and maps to ops_group_member observations.
func ImportAzureADGroupMemberships(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client, err := newAzureADClient(config)
	if err != nil {
		return results, fmt.Errorf("azure ad client init: %w", err)
	}

	// first collect all groups
	type groupRef struct {
		id   string
		name string
	}
	var allGroups []groupRef

	nextURL := client.baseURL + "/v1.0/groups?$top=" + fmt.Sprintf("%d", config.BatchSize) +
		"&$select=id,displayName"

	for nextURL != "" {
		body, next, err := client.graphGet(nextURL)
		if err != nil {
			return results, fmt.Errorf("azure ad list groups for memberships: %w", err)
		}

		items, ok := body["value"].([]interface{})
		if !ok {
			break
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			name := azGetString(m, "displayName")
			if name == "" || isExcludedGroup(name, config.ExcludeGroups) {
				continue
			}
			allGroups = append(allGroups, groupRef{
				id:   azGetString(m, "id"),
				name: name,
			})
		}

		nextURL = next
	}

	// for each group, get members
	for _, group := range allGroups {
		memberURL := fmt.Sprintf("%s/v1.0/groups/%s/members?$top=%d&$select=id,userPrincipalName,displayName,userType,accountEnabled",
			client.baseURL, group.id, config.BatchSize)

		for memberURL != "" {
			body, next, err := client.graphGet(memberURL)
			if err != nil {
				results = append(results, Observation{
					EntityType: "ops_group_member",
					EntityID:   fmt.Sprintf("%s_error", group.id),
					StateKey:   "azuread_group_member_error",
					Value:      fmt.Sprintf("failed to read members: %v", err),
					DataJSON: map[string]interface{}{
						"group_id":   group.id,
						"group_name": group.name,
						"error":      err.Error(),
					},
				})
				break
			}

			items, ok := body["value"].([]interface{})
			if !ok {
				break
			}

			for _, item := range items {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				// only include user-type members, skip nested groups and service principals
				odataType := azGetString(m, "@odata.type")
				if odataType != "" && odataType != "#microsoft.graph.user" {
					continue
				}

				memberUPN := azGetString(m, "userPrincipalName")
				if memberUPN == "" {
					memberUPN = azGetString(m, "id")
				}

				membershipID := fmt.Sprintf("%s_%s", group.name, memberUPN)
				obs := Observation{
					EntityType: "ops_group_member",
					EntityID:   membershipID,
					StateKey:   "azuread_group_member",
					Value:      fmt.Sprintf("%s in %s", memberUPN, group.name),
					DataJSON: map[string]interface{}{
						"group_name":          group.name,
						"group_id":            group.id,
						"user_principal_name": memberUPN,
						"user_id":             azGetString(m, "id"),
						"display_name":        azGetString(m, "displayName"),
						"user_type":           azGetString(m, "userType"),
						"account_enabled":     azGetBool(m, "accountEnabled"),
						"source":              "azuread",
						"tenant_id":           config.TenantID,
					},
				}
				results = append(results, obs)
			}

			memberURL = next
		}
	}

	return results, nil
}

// azureADClient wraps Microsoft Graph API access with OAuth2 client credentials.
type azureADClient struct {
	baseURL     string
	accessToken string
	retryConfig runner.RetryConfig
}

func newAzureADClient(config *ImportConfig) (*azureADClient, error) {
	if config.TenantID == "" {
		return nil, fmt.Errorf("azure ad tenant_id not configured")
	}
	if config.ClientID == "" || config.ClientSecret == "" {
		return nil, fmt.Errorf("azure ad client_id and client_secret required")
	}

	retryConfig := runner.RetryConfig{
		MaxAttempts:  config.MaxRetries,
		BaseDelay:    time.Second,
		Multiplier:   2.0,
		JitterFrac:   0.25,
		MaxTotalTime: 30 * time.Second,
	}

	// acquire access token via OAuth2 client credentials flow
	token, err := acquireAzureToken(config.TenantID, config.ClientID, config.ClientSecret, retryConfig)
	if err != nil {
		return nil, fmt.Errorf("acquiring azure ad token: %w", err)
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://graph.microsoft.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &azureADClient{
		baseURL:     baseURL,
		accessToken: token,
		retryConfig: retryConfig,
	}, nil
}

// graphGet performs an authenticated GET against Microsoft Graph API.
// Returns the response body and the next page URL (from @odata.nextLink).
func (c *azureADClient) graphGet(url string) (map[string]interface{}, string, error) {
	var body map[string]interface{}
	var nextLink string

	err := runner.WithRetry(c.retryConfig, func() error {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("ConsistencyLevel", "eventual")

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("executing request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 429 {
			return fmt.Errorf("graph API rate limited (429) for %s", url)
		}

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("graph API returned status %d for %s: %s", resp.StatusCode, url, string(bodyBytes))
		}

		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}

		if next, ok := body["@odata.nextLink"].(string); ok {
			nextLink = next
		}

		return nil
	})

	return body, nextLink, err
}

// acquireAzureToken gets an OAuth2 access token via client credentials flow.
func acquireAzureToken(tenantID string, clientID string, clientSecret string, retryConfig runner.RetryConfig) (string, error) {
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID)

	var token string

	err := runner.WithRetry(retryConfig, func() error {
		formData := fmt.Sprintf("client_id=%s&client_secret=%s&scope=%s&grant_type=client_credentials",
			clientID, clientSecret, "https://graph.microsoft.com/.default")

		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(formData))
		if err != nil {
			return fmt.Errorf("creating token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("executing token request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(bodyBytes))
		}

		var body map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return fmt.Errorf("decoding token response: %w", err)
		}

		accessToken, ok := body["access_token"].(string)
		if !ok || accessToken == "" {
			return fmt.Errorf("no access_token in token response")
		}
		token = accessToken
		return nil
	})

	return token, err
}

// classifyAzureGroup determines the group classification from its properties.
func classifyAzureGroup(m map[string]interface{}, groupTypes []string) string {
	for _, gt := range groupTypes {
		if gt == "Unified" {
			return "microsoft_365"
		}
		if gt == "DynamicMembership" {
			return "dynamic"
		}
	}
	if azGetBool(m, "securityEnabled") {
		if azGetBool(m, "mailEnabled") {
			return "mail_enabled_security"
		}
		return "security"
	}
	if azGetBool(m, "mailEnabled") {
		return "distribution"
	}
	return "other"
}

// azGetString safely extracts a string from a map.
func azGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// azGetBool safely extracts a bool from a map.
func azGetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// azGetStringList safely extracts a string slice from a map.
func azGetStringList(m map[string]interface{}, key string) []string {
	raw, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
