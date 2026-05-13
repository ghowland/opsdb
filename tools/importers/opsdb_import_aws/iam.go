// === importers/opsdb-import-aws/iam.go ===
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// ImportIAM reads IAM roles from AWS API. IAM is global, not regional.
func ImportIAM(importCfg *ImportConfig) ([]Observation, error) {
	var results []Observation

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return results, fmt.Errorf("loading aws config for iam: %w", err)
	}

	client := iam.NewFromConfig(cfg)

	rolePaginator := iam.NewListRolesPaginator(client, &iam.ListRolesInput{
		MaxItems: aws.Int32(int32(importCfg.BatchSize)),
	})

	for rolePaginator.HasMorePages() {
		page, err := rolePaginator.NextPage(ctx)
		if err != nil {
			return results, fmt.Errorf("iam ListRoles page: %w", err)
		}

		for _, role := range page.Roles {
			obs, err := mapIAMRole(client, ctx, role)
			if err != nil {
				// log and continue rather than failing entire import
				results = append(results, Observation{
					EntityType: "cloud_resource",
					EntityID:   aws.ToString(role.RoleName),
					StateKey:   "aws_iam_role_error",
					Value:      fmt.Sprintf("failed to map role: %v", err),
					DataJSON: map[string]interface{}{
						"cloud_resource_type": "iam_role",
						"role_name":           aws.ToString(role.RoleName),
						"error":               err.Error(),
					},
				})
				continue
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

func mapIAMRole(client *iam.Client, ctx context.Context, role iamtypes.Role) (Observation, error) {
	roleName := aws.ToString(role.RoleName)
	roleArn := aws.ToString(role.Arn)

	// get attached policy count
	attachedPolicies, err := listAttachedPolicies(client, ctx, roleName)
	if err != nil {
		return Observation{}, fmt.Errorf("listing attached policies for %s: %w", roleName, err)
	}

	policyNames := make([]string, 0, len(attachedPolicies))
	policyArns := make([]string, 0, len(attachedPolicies))
	for _, p := range attachedPolicies {
		policyNames = append(policyNames, aws.ToString(p.PolicyName))
		policyArns = append(policyArns, aws.ToString(p.PolicyArn))
	}

	// get inline policy count
	inlinePolicies, err := listInlinePolicies(client, ctx, roleName)
	if err != nil {
		return Observation{}, fmt.Errorf("listing inline policies for %s: %w", roleName, err)
	}

	// parse trust policy to extract principals
	trustPrincipals := extractTrustPrincipals(role.AssumeRolePolicyDocument)

	var createDate string
	if role.CreateDate != nil {
		createDate = role.CreateDate.Format(time.RFC3339)
	}

	var lastUsedTime string
	var lastUsedRegion string
	if role.RoleLastUsed != nil {
		if role.RoleLastUsed.LastUsedDate != nil {
			lastUsedTime = role.RoleLastUsed.LastUsedDate.Format(time.RFC3339)
		}
		lastUsedRegion = aws.ToString(role.RoleLastUsed.Region)
	}

	var maxSessionDuration int
	if role.MaxSessionDuration != nil {
		maxSessionDuration = int(*role.MaxSessionDuration)
	}

	tags := make(map[string]string, len(role.Tags))
	for _, tag := range role.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return Observation{
		EntityType: "cloud_resource",
		EntityID:   roleName,
		StateKey:   "aws_iam_role",
		Value:      roleName,
		DataJSON: map[string]interface{}{
			"cloud_resource_type":     "iam_role",
			"role_name":               roleName,
			"role_arn":                 roleArn,
			"role_id":                 aws.ToString(role.RoleId),
			"path":                    aws.ToString(role.Path),
			"description":             aws.ToString(role.Description),
			"created_time":            createDate,
			"max_session_duration":    maxSessionDuration,
			"attached_policy_count":   len(attachedPolicies),
			"attached_policy_names":   policyNames,
			"attached_policy_arns":    policyArns,
			"inline_policy_count":     len(inlinePolicies),
			"inline_policy_names":     inlinePolicies,
			"trust_principals":        trustPrincipals,
			"last_used_time":          lastUsedTime,
			"last_used_region":        lastUsedRegion,
			"tags":                    tags,
		},
	}, nil
}

func listAttachedPolicies(client *iam.Client, ctx context.Context, roleName string) ([]iamtypes.AttachedPolicy, error) {
	var policies []iamtypes.AttachedPolicy

	paginator := iam.NewListAttachedRolePoliciesPaginator(client, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return policies, err
		}
		policies = append(policies, page.AttachedPolicies...)
	}

	return policies, nil
}

func listInlinePolicies(client *iam.Client, ctx context.Context, roleName string) ([]string, error) {
	var names []string

	paginator := iam.NewListRolePoliciesPaginator(client, &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return names, err
		}
		names = append(names, page.PolicyNames...)
	}

	return names, nil
}

// extractTrustPrincipals parses the assume role policy document and extracts
// principal identifiers. Returns a flat list of principal strings rather than
// the full policy JSON.
func extractTrustPrincipals(policyDoc *string) []string {
	if policyDoc == nil {
		return nil
	}

	docStr := aws.ToString(policyDoc)

	// trust policy documents are URL-encoded in the API response
	decoded, err := url.QueryUnescape(docStr)
	if err != nil {
		decoded = docStr
	}

	var policy trustPolicy
	if err := json.Unmarshal([]byte(decoded), &policy); err != nil {
		return []string{"parse_error"}
	}

	seen := make(map[string]bool)
	var principals []string

	for _, stmt := range policy.Statement {
		if stmt.Effect != "Allow" {
			continue
		}
		for _, p := range flattenPrincipal(stmt.Principal) {
			if !seen[p] {
				seen[p] = true
				principals = append(principals, p)
			}
		}
	}

	return principals
}

// trustPolicy is a minimal representation of an IAM trust policy.
type trustPolicy struct {
	Statement []trustStatement `json:"Statement"`
}

type trustStatement struct {
	Effect    string      `json:"Effect"`
	Principal interface{} `json:"Principal"`
}

// flattenPrincipal extracts principal strings from the various forms
// the Principal field can take in IAM policy documents.
func flattenPrincipal(principal interface{}) []string {
	if principal == nil {
		return nil
	}

	// Principal can be "*"
	if s, ok := principal.(string); ok {
		return []string{s}
	}

	// Principal can be {"Service": "..."} or {"AWS": "..."} or {"Federated": "..."}
	// Each value can be a string or []string
	m, ok := principal.(map[string]interface{})
	if !ok {
		return nil
	}

	var result []string
	for principalType, value := range m {
		prefix := principalType + ":"
		switch v := value.(type) {
		case string:
			result = append(result, prefix+v)
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					result = append(result, prefix+s)
				}
			}
		}
	}

	return result
}
