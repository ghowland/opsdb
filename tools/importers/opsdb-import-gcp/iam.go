package gcp

import "fmt"

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
	OldestKeyAge   string // RFC3339 of oldest key creation, empty if no user-managed keys
	ProjectRoles   []string // roles granted at project level
	Oauth2ClientID string
}

// listServiceAccounts reads all service accounts in a project and
// fetches per-account key and binding metadata.
func listServiceAccounts(project string) ([]gcpServiceAccount, error) {
	// TODO: create IAM service client using application default credentials
	// TODO: call projects.serviceAccounts.list(project)
	// TODO: handle pagination via NextPageToken
	// TODO: for each service account in response:
	//   extract Email, UniqueId, DisplayName, Description, Disabled, Oauth2ClientId
	//   call projects.serviceAccounts.keys.list(account name)
	//     filter to USER_MANAGED keys
	//     count keys, find oldest ValidAfterTime
	//   call projects.getIamPolicy(project)
	//     (cache once per project, not per account)
	//     find bindings where member matches this service account
	//     extract role names
	// TODO: handle AccessDenied gracefully per account
	// TODO: return accounts
	return nil, nil
}

// MapGCPServiceAccount transforms a raw GCP service account into an OpsDB
// observation. Flattens per-account metadata into cloud_data_json.
// Individual key details and per-resource IAM bindings are not expanded —
// the count and role summary provide the operational signal.
func MapGCPServiceAccount(project string, acct gcpServiceAccount) Observation {
	dataJSON := map[string]interface{}{
		"project":           project,
		"email":             acct.Email,
		"unique_id":         acct.UniqueID,
		"display_name":      acct.DisplayName,
		"description":       acct.Description,
		"is_disabled":       acct.Disabled,
		"user_managed_key_count": acct.KeyCount,
		"project_role_count": len(acct.ProjectRoles),
		"oauth2_client_id":  acct.Oauth2ClientID,
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
