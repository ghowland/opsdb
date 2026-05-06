// === importers/opsdb-import-identity/okta.go ===
package identity

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

// ImportConfig holds identity importer configuration shared across backends.
type ImportConfig struct {
	Backend      string
	BaseURL      string
	APIToken     string
	BatchSize    int
	MaxRetries   int
	Domain       string
	TenantID     string
	ClientID     string
	ClientSecret string
	LDAPBaseDN   string
	LDAPBindDN   string
	LDAPBindPass string
	UserFilter   string
	GroupFilter  string
	ExcludeGroups []string
}

// Observation is the observation structure for identity importers.
type Observation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportOktaUsers reads users from the Okta API and maps to ops_user observations.
func ImportOktaUsers(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOktaClient(config)
	nextURL := fmt.Sprintf("%s/api/v1/users?limit=%d", client.baseURL, config.BatchSize)

	for nextURL != "" {
		users, next, err := client.getUsers(nextURL)
		if err != nil {
			return results, fmt.Errorf("okta list users: %w", err)
		}

		for _, user := range users {
			profile := user.Profile
			uid := oktaGetString(profile, "login")
			if uid == "" {
				uid = user.ID
			}

			displayName := oktaGetString(profile, "displayName")
			if displayName == "" {
				first := oktaGetString(profile, "firstName")
				last := oktaGetString(profile, "lastName")
				displayName = strings.TrimSpace(first + " " + last)
			}

			isActive := user.Status == "ACTIVE" || user.Status == "PASSWORD_EXPIRED" || user.Status == "RECOVERY"

			obs := Observation{
				EntityType: "ops_user",
				EntityID:   uid,
				StateKey:   "okta_user",
				Value:      displayName,
				DataJSON: map[string]interface{}{
					"okta_id":          user.ID,
					"login":            oktaGetString(profile, "login"),
					"email":            oktaGetString(profile, "email"),
					"display_name":     displayName,
					"first_name":       oktaGetString(profile, "firstName"),
					"last_name":        oktaGetString(profile, "lastName"),
					"title":            oktaGetString(profile, "title"),
					"department":       oktaGetString(profile, "department"),
					"organization":     oktaGetString(profile, "organization"),
					"mobile_phone":     oktaGetString(profile, "mobilePhone"),
					"status":           user.Status,
					"is_active":        isActive,
					"created_time":     user.Created,
					"last_login_time":  user.LastLogin,
					"last_updated_time": user.LastUpdated,
					"status_changed_time": user.StatusChanged,
					"source":           "okta",
					"okta_domain":      config.Domain,
				},
			}
			results = append(results, obs)
		}

		nextURL = next
	}

	return results, nil
}

// ImportOktaGroups reads groups from the Okta API and maps to ops_group observations.
func ImportOktaGroups(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOktaClient(config)
	nextURL := fmt.Sprintf("%s/api/v1/groups?limit=%d", client.baseURL, config.BatchSize)

	for nextURL != "" {
		groups, next, err := client.getGroups(nextURL)
		if err != nil {
			return results, fmt.Errorf("okta list groups: %w", err)
		}

		for _, group := range groups {
			name := oktaGetString(group.Profile, "name")
			if name == "" {
				continue
			}

			if isExcludedGroup(name, config.ExcludeGroups) {
				continue
			}

			obs := Observation{
				EntityType: "ops_group",
				EntityID:   name,
				StateKey:   "okta_group",
				Value:      name,
				DataJSON: map[string]interface{}{
					"okta_id":          group.ID,
					"name":             name,
					"description":      oktaGetString(group.Profile, "description"),
					"group_type":       group.Type,
					"member_count":     group.MemberCount,
					"created_time":     group.Created,
					"last_updated_time": group.LastUpdated,
					"source":           "okta",
					"okta_domain":      config.Domain,
				},
			}
			results = append(results, obs)
		}

		nextURL = next
	}

	return results, nil
}

// ImportOktaGroupMemberships reads group memberships from the Okta API
// and maps to ops_group_member observations.
func ImportOktaGroupMemberships(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOktaClient(config)

	// first get all groups
	nextURL := fmt.Sprintf("%s/api/v1/groups?limit=%d", client.baseURL, config.BatchSize)
	var allGroups []oktaGroup

	for nextURL != "" {
		groups, next, err := client.getGroups(nextURL)
		if err != nil {
			return results, fmt.Errorf("okta list groups for memberships: %w", err)
		}
		allGroups = append(allGroups, groups...)
		nextURL = next
	}

	// for each group, get members
	for _, group := range allGroups {
		groupName := oktaGetString(group.Profile, "name")
		if groupName == "" {
			continue
		}

		if isExcludedGroup(groupName, config.ExcludeGroups) {
			continue
		}

		memberURL := fmt.Sprintf("%s/api/v1/groups/%s/users?limit=%d", client.baseURL, group.ID, config.BatchSize)

		for memberURL != "" {
			members, next, err := client.getGroupMembers(memberURL)
			if err != nil {
				// log and continue with next group rather than failing entirely
				results = append(results, Observation{
					EntityType: "ops_group_member",
					EntityID:   fmt.Sprintf("%s_error", group.ID),
					StateKey:   "okta_group_member_error",
					Value:      fmt.Sprintf("failed to read members: %v", err),
					DataJSON: map[string]interface{}{
						"group_id":   group.ID,
						"group_name": groupName,
						"error":      err.Error(),
					},
				})
				break
			}

			for _, member := range members {
				memberLogin := oktaGetString(member.Profile, "login")
				if memberLogin == "" {
					memberLogin = member.ID
				}

				membershipID := fmt.Sprintf("%s_%s", groupName, memberLogin)
				obs := Observation{
					EntityType: "ops_group_member",
					EntityID:   membershipID,
					StateKey:   "okta_group_member",
					Value:      fmt.Sprintf("%s in %s", memberLogin, groupName),
					DataJSON: map[string]interface{}{
						"group_name":    groupName,
						"group_id":      group.ID,
						"user_login":    memberLogin,
						"user_id":       member.ID,
						"user_email":    oktaGetString(member.Profile, "email"),
						"user_status":   member.Status,
						"source":        "okta",
						"okta_domain":   config.Domain,
					},
				}
				results = append(results, obs)
			}

			memberURL = next
		}
	}

	return results, nil
}

// oktaClient wraps Okta API access with authentication and retry.
type oktaClient struct {
	apiToken    string
	baseURL     string
	retryConfig runner.RetryConfig
}

func newOktaClient(config *ImportConfig) *oktaClient {
	baseURL := config.BaseURL
	if baseURL == "" && config.Domain != "" {
		baseURL = fmt.Sprintf("https://%s", config.Domain)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &oktaClient{
		apiToken: config.APIToken,
		baseURL:  baseURL,
		retryConfig: runner.RetryConfig{
			MaxAttempts:  config.MaxRetries,
			BaseDelay:    time.Second,
			Multiplier:   2.0,
			JitterFrac:   0.25,
			MaxTotalTime: 30 * time.Second,
		},
	}
}

// oktaUser represents a user from the Okta API.
type oktaUser struct {
	ID            string
	Status        string
	Created       string
	LastLogin     string
	LastUpdated   string
	StatusChanged string
	Profile       map[string]interface{}
}

// oktaGroup represents a group from the Okta API.
type oktaGroup struct {
	ID          string
	Type        string
	Created     string
	LastUpdated string
	MemberCount int
	Profile     map[string]interface{}
}

// getUsers fetches a page of users from the given URL.
func (c *oktaClient) getUsers(url string) ([]oktaUser, string, error) {
	var users []oktaUser
	var nextLink string

	err := runner.WithRetry(c.retryConfig, func() error {
		body, headers, err := oktaGet(url, c.apiToken)
		if err != nil {
			return err
		}

		nextLink = parseOktaNextLink(headers)

		items, ok := body.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from users endpoint")
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			user := oktaUser{
				ID:            oktaGetString(m, "id"),
				Status:        oktaGetString(m, "status"),
				Created:       oktaGetString(m, "created"),
				LastLogin:     oktaGetString(m, "lastLogin"),
				LastUpdated:   oktaGetString(m, "lastUpdated"),
				StatusChanged: oktaGetString(m, "statusChanged"),
			}

			if profile, ok := m["profile"].(map[string]interface{}); ok {
				user.Profile = profile
			}

			users = append(users, user)
		}
		return nil
	})

	return users, nextLink, err
}

// getGroups fetches a page of groups from the given URL.
func (c *oktaClient) getGroups(url string) ([]oktaGroup, string, error) {
	var groups []oktaGroup
	var nextLink string

	err := runner.WithRetry(c.retryConfig, func() error {
		body, headers, err := oktaGet(url, c.apiToken)
		if err != nil {
			return err
		}

		nextLink = parseOktaNextLink(headers)

		items, ok := body.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from groups endpoint")
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			group := oktaGroup{
				ID:          oktaGetString(m, "id"),
				Type:        oktaGetString(m, "type"),
				Created:     oktaGetString(m, "created"),
				LastUpdated: oktaGetString(m, "lastUpdated"),
			}

			if profile, ok := m["profile"].(map[string]interface{}); ok {
				group.Profile = profile
			}

			// Okta doesn't return member count in list, but some endpoints do
			if embedded, ok := m["_embedded"].(map[string]interface{}); ok {
				if stats, ok := embedded["stats"].(map[string]interface{}); ok {
					if count, ok := stats["usersCount"].(float64); ok {
						group.MemberCount = int(count)
					}
				}
			}

			groups = append(groups, group)
		}
		return nil
	})

	return groups, nextLink, err
}

// getGroupMembers fetches a page of group members from the given URL.
func (c *oktaClient) getGroupMembers(url string) ([]oktaUser, string, error) {
	var members []oktaUser
	var nextLink string

	err := runner.WithRetry(c.retryConfig, func() error {
		body, headers, err := oktaGet(url, c.apiToken)
		if err != nil {
			return err
		}

		nextLink = parseOktaNextLink(headers)

		items, ok := body.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from group members endpoint")
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			member := oktaUser{
				ID:     oktaGetString(m, "id"),
				Status: oktaGetString(m, "status"),
			}

			if profile, ok := m["profile"].(map[string]interface{}); ok {
				member.Profile = profile
			}

			members = append(members, member)
		}
		return nil
	})

	return members, nextLink, err
}

// oktaGet performs an authenticated GET request against the Okta API.
func oktaGet(url string, apiToken string) (interface{}, http.Header, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "SSWS "+apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, resp.Header, fmt.Errorf("okta rate limited (429) for %s", url)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, resp.Header, fmt.Errorf("okta API returned status %d for %s: %s", resp.StatusCode, url, string(bodyBytes))
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, resp.Header, fmt.Errorf("decoding response: %w", err)
	}

	return body, resp.Header, nil
}

// parseOktaNextLink extracts the next page URL from Okta's Link header.
// Okta uses RFC 5988 Link headers for pagination:
// Link: <https://example.okta.com/api/v1/users?after=xyz>; rel="next"
func parseOktaNextLink(headers http.Header) string {
	links := headers.Values("Link")
	for _, link := range links {
		parts := strings.Split(link, ";")
		if len(parts) < 2 {
			continue
		}
		relPart := strings.TrimSpace(parts[1])
		if relPart == `rel="next"` {
			urlPart := strings.TrimSpace(parts[0])
			urlPart = strings.TrimPrefix(urlPart, "<")
			urlPart = strings.TrimSuffix(urlPart, ">")
			return urlPart
		}
	}
	return ""
}

// oktaGetString safely extracts a string from a map.
func oktaGetString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
