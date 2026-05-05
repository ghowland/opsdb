package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	iam "google.golang.org/api/iam/v1"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"
)

// ImportIAM reads GCP IAM service accounts from the GCP API across all
// configured projects. For each service account, fetches key count and
// summarizes role bindings. Maps to cloud_resource with service_account
// discriminator type.
func ImportIAM(config *GCPImportConfig) ([]Observation, error) {
	var observations []Observation

	for _, project := range config.Projects {
		accounts, err := listServiceAccounts(project)
		if err != nil {
			return observations, fmt.Errorf("listing service accounts in project %s: %w", project, err)
		}

		for _, acct := range accounts {
			obs := MapGCPServiceAccount(project, acct)
			observations = append(observations, obs)

			if len(observations) >= config.BatchSize {
				return observations, nil
			}
		}
	}

	return observations, nil
}

// gcpServiceAccount holds the raw fields read from the GCP IAM API.
type gcpServiceAccount struct {
	Email          string
	UniqueID       string
	DisplayName    string
	Description    string
	Disabled       bool
	KeyCount       int
	OldestKeyAge   string
	ProjectRoles   []string
	Oauth2ClientID string
}

// listServiceAccounts reads all service accounts in a project, fetches
// per-account key metadata, and resolves project-level role bindings.
func listServiceAccounts(project string) ([]gcpServiceAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	iamSvc, err := iam.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating IAM client: %w", err)
	}

	// Fetch project IAM policy once for all accounts in this project.
	projectRoleMap, err := getProjectRoleMap(ctx, project)
	if err != nil {
		// Non-fatal: we can still list accounts without role bindings.
		projectRoleMap = make(map[string][]string)
	}

	var accounts []gcpServiceAccount
	projectResource := fmt.Sprintf("projects/%s", project)
	pageToken := ""

	for {
		call := iamSvc.Projects.ServiceAccounts.List(projectResource)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Context(ctx).Do()
		if err != nil {
			return accounts, fmt.Errorf("listing service accounts: %w", err)
		}

		for _, sa := range resp.Accounts {
			acct := gcpServiceAccount{
				Email:          sa.Email,
				UniqueID:       sa.UniqueId,
				DisplayName:    sa.DisplayName,
				Description:    sa.Description,
				Disabled:       sa.Disabled,
				Oauth2ClientID: sa.Oauth2ClientId,
			}

			// Fetch keys for this service account.
			keyCount, oldestKey, err := getServiceAccountKeys(ctx, iamSvc, sa.Name)
			if err != nil {
				// Non-fatal: permission denied on keys is common for
				// cross-project service accounts. Record zero keys.
				keyCount = 0
				oldestKey = ""
			}
			acct.KeyCount = keyCount
			acct.OldestKeyAge = oldestKey

			// Look up project-level roles from the cached policy.
			memberKey := fmt.Sprintf("serviceAccount:%s", sa.Email)
			if roles, ok := projectRoleMap[memberKey]; ok {
				acct.ProjectRoles = roles
			}

			accounts = append(accounts, acct)
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return accounts, nil
}

// getServiceAccountKeys lists user-managed keys for a service account.
// Returns the count of user-managed keys and the RFC3339 creation time
// of the oldest key. System-managed keys are excluded.
func getServiceAccountKeys(ctx context.Context, iamSvc *iam.Service, accountName string) (int, string, error) {
	resp, err := iamSvc.Projects.ServiceAccounts.Keys.List(accountName).
		KeyTypes("USER_MANAGED").
		Context(ctx).
		Do()
	if err != nil {
		return 0, "", fmt.Errorf("listing keys for %s: %w", accountName, err)
	}

	count := len(resp.Keys)
	oldestTime := ""

	for _, key := range resp.Keys {
		if key.ValidAfterTime != "" {
			if oldestTime == "" || key.ValidAfterTime < oldestTime {
				oldestTime = key.ValidAfterTime
			}
		}
	}

	return count, oldestTime, nil
}

// getProjectRoleMap fetches the IAM policy for a project and builds a map
// from member identity to the list of roles granted at project level.
// Called once per project, not per service account.
func getProjectRoleMap(ctx context.Context, project string) (map[string][]string, error) {
	crmSvc, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating resource manager client: %w", err)
	}

	policy, err := crmSvc.Projects.GetIamPolicy(project,
		&cloudresourcemanager.GetIamPolicyRequest{}).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("getting IAM policy for project %s: %w", project, err)
	}

	roleMap := make(map[string][]string)
	for _, binding := range policy.Bindings {
		for _, member := range binding.Members {
			// Only track service account members.
			if strings.HasPrefix(member, "serviceAccount:") {
				roleMap[member] = append(roleMap[member], binding.Role)
			}
		}
	}

	return roleMap, nil
}

// MapGCPServiceAccount transforms a raw GCP service account into an OpsDB
// observation. Flattens per-account metadata into cloud_data_json.
// Individual key details and per-resource IAM bindings are not expanded —
// the count and role summary provide the operational signal.
func MapGCPServiceAccount(project string, acct gcpServiceAccount) Observation {
	dataJSON := map[string]interface{}{
		"project":                project,
		"email":                  acct.Email,
		"unique_id":              acct.UniqueID,
		"display_name":           acct.DisplayName,
		"description":            acct.Description,
		"is_disabled":            acct.Disabled,
		"user_managed_key_count": acct.KeyCount,
		"project_role_count":     len(acct.ProjectRoles),
		"oauth2_client_id":       acct.Oauth2ClientID,
	}

	if acct.OldestKeyAge != "" {
		dataJSON["oldest_key_created_time"] = acct.OldestKeyAge
	}

	if len(acct.ProjectRoles) > 0 {
		dataJSON["project_roles"] = acct.ProjectRoles
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   fmt.Sprintf("gcp:%s:iam:sa:%s", project, acct.UniqueID),
		StateKey:   "service_account",
		Value:      disabledToStatus(acct.Disabled),
		DataJSON:   dataJSON,
	}
}

// disabledToStatus converts the disabled boolean to a status string.
func disabledToStatus(disabled bool) string {
	if disabled {
		return "disabled"
	}
	return "active"
}