// === importers/opsdb_import_identity/ldap.go ===
package identity

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// ImportLDAPUsers reads user entries from LDAP and maps to ops_user observations.
func ImportLDAPUsers(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	conn, err := connectLDAP(config)
	if err != nil {
		return results, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	userBaseDN := config.LDAPBaseDN
	if config.UserFilter == "" {
		config.UserFilter = "(objectClass=inetOrgPerson)"
	}

	userAttrs := []string{
		"uid", "cn", "sn", "givenName", "mail", "displayName",
		"title", "department", "employeeNumber", "telephoneNumber",
		"accountStatus", "userAccountControl", "createTimestamp",
		"modifyTimestamp", "memberOf",
	}

	searchReq := ldap.NewSearchRequest(
		userBaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		30,
		false,
		config.UserFilter,
		userAttrs,
		[]ldap.Control{ldap.NewControlPaging(uint32(config.BatchSize))},
	)

	for {
		result, err := conn.SearchWithPaging(searchReq, uint32(config.BatchSize))
		if err != nil {
			return results, fmt.Errorf("ldap user search: %w", err)
		}

		for _, entry := range result.Entries {
			uid := entry.GetAttributeValue("uid")
			if uid == "" {
				uid = entry.GetAttributeValue("cn")
			}
			if uid == "" {
				continue
			}

			mail := entry.GetAttributeValue("mail")
			displayName := entry.GetAttributeValue("displayName")
			if displayName == "" {
				displayName = entry.GetAttributeValue("cn")
			}

			accountActive := isLDAPAccountActive(entry)
			memberOfGroups := entry.GetAttributeValues("memberOf")
			groupNames := make([]string, 0, len(memberOfGroups))
			for _, dn := range memberOfGroups {
				groupNames = append(groupNames, extractCNFromDN(dn))
			}

			obs := Observation{
				EntityType: "ops_user",
				EntityID:   uid,
				StateKey:   "ldap_user",
				Value:      displayName,
				DataJSON: map[string]interface{}{
					"uid":              uid,
					"cn":               entry.GetAttributeValue("cn"),
					"given_name":       entry.GetAttributeValue("givenName"),
					"surname":          entry.GetAttributeValue("sn"),
					"display_name":     displayName,
					"email":            mail,
					"title":            entry.GetAttributeValue("title"),
					"department":       entry.GetAttributeValue("department"),
					"employee_number":  entry.GetAttributeValue("employeeNumber"),
					"telephone":        entry.GetAttributeValue("telephoneNumber"),
					"is_active":        accountActive,
					"dn":               entry.DN,
					"member_of_count":  len(memberOfGroups),
					"member_of_groups": groupNames,
					"created_time":     entry.GetAttributeValue("createTimestamp"),
					"modified_time":    entry.GetAttributeValue("modifyTimestamp"),
					"source":           "ldap",
					"ldap_server":      config.BaseURL,
				},
			}
			results = append(results, obs)
		}

		// SearchWithPaging handles all pages internally, so we break after one call
		break
	}

	return results, nil
}

// ImportLDAPGroups reads group entries from LDAP and maps to ops_group observations.
func ImportLDAPGroups(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	conn, err := connectLDAP(config)
	if err != nil {
		return results, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	groupBaseDN := config.LDAPBaseDN
	groupFilter := config.GroupFilter
	if groupFilter == "" {
		groupFilter = "(|(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))"
	}

	groupAttrs := []string{
		"cn", "description", "member", "memberUid", "uniqueMember",
		"createTimestamp", "modifyTimestamp",
	}

	result, err := conn.SearchWithPaging(
		ldap.NewSearchRequest(
			groupBaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0,
			30,
			false,
			groupFilter,
			groupAttrs,
			nil,
		),
		uint32(config.BatchSize),
	)
	if err != nil {
		return results, fmt.Errorf("ldap group search: %w", err)
	}

	for _, entry := range result.Entries {
		cn := entry.GetAttributeValue("cn")
		if cn == "" {
			continue
		}

		if isExcludedGroup(cn, config.ExcludeGroups) {
			continue
		}

		members := entry.GetAttributeValues("member")
		uniqueMembers := entry.GetAttributeValues("uniqueMember")
		memberUids := entry.GetAttributeValues("memberUid")
		totalMembers := len(members) + len(uniqueMembers) + len(memberUids)

		obs := Observation{
			EntityType: "ops_group",
			EntityID:   cn,
			StateKey:   "ldap_group",
			Value:      cn,
			DataJSON: map[string]interface{}{
				"cn":            cn,
				"description":   entry.GetAttributeValue("description"),
				"member_count":  totalMembers,
				"dn":            entry.DN,
				"created_time":  entry.GetAttributeValue("createTimestamp"),
				"modified_time": entry.GetAttributeValue("modifyTimestamp"),
				"source":        "ldap",
				"ldap_server":   config.BaseURL,
			},
		}
		results = append(results, obs)
	}

	return results, nil
}

// ImportLDAPGroupMemberships reads group membership from LDAP and maps to
// ops_group_member observations.
func ImportLDAPGroupMemberships(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	conn, err := connectLDAP(config)
	if err != nil {
		return results, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	groupBaseDN := config.LDAPBaseDN
	groupFilter := config.GroupFilter
	if groupFilter == "" {
		groupFilter = "(|(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))"
	}

	groupAttrs := []string{"cn", "member", "memberUid", "uniqueMember"}

	result, err := conn.SearchWithPaging(
		ldap.NewSearchRequest(
			groupBaseDN,
			ldap.ScopeWholeSubtree,
			ldap.NeverDerefAliases,
			0,
			30,
			false,
			groupFilter,
			groupAttrs,
			nil,
		),
		uint32(config.BatchSize),
	)
	if err != nil {
		return results, fmt.Errorf("ldap group membership search: %w", err)
	}

	for _, entry := range result.Entries {
		groupCN := entry.GetAttributeValue("cn")
		if groupCN == "" {
			continue
		}

		if isExcludedGroup(groupCN, config.ExcludeGroups) {
			continue
		}

		// handle member attribute (DN-based membership)
		for _, memberDN := range entry.GetAttributeValues("member") {
			memberUID := extractUIDFromDN(memberDN)
			if memberUID == "" {
				memberUID = extractCNFromDN(memberDN)
			}
			if memberUID == "" {
				continue
			}

			membershipID := fmt.Sprintf("%s_%s", groupCN, memberUID)
			obs := Observation{
				EntityType: "ops_group_member",
				EntityID:   membershipID,
				StateKey:   "ldap_group_member",
				Value:      fmt.Sprintf("%s in %s", memberUID, groupCN),
				DataJSON: map[string]interface{}{
					"group_name":  groupCN,
					"user_uid":    memberUID,
					"member_dn":   memberDN,
					"source":      "ldap",
					"ldap_server": config.BaseURL,
				},
			}
			results = append(results, obs)
		}

		// handle uniqueMember attribute (DN-based, used by groupOfUniqueNames)
		for _, memberDN := range entry.GetAttributeValues("uniqueMember") {
			memberUID := extractUIDFromDN(memberDN)
			if memberUID == "" {
				memberUID = extractCNFromDN(memberDN)
			}
			if memberUID == "" {
				continue
			}

			membershipID := fmt.Sprintf("%s_%s", groupCN, memberUID)
			obs := Observation{
				EntityType: "ops_group_member",
				EntityID:   membershipID,
				StateKey:   "ldap_group_member",
				Value:      fmt.Sprintf("%s in %s", memberUID, groupCN),
				DataJSON: map[string]interface{}{
					"group_name":  groupCN,
					"user_uid":    memberUID,
					"member_dn":   memberDN,
					"source":      "ldap",
					"ldap_server": config.BaseURL,
				},
			}
			results = append(results, obs)
		}

		// handle memberUid attribute (UID-based, used by posixGroup)
		for _, uid := range entry.GetAttributeValues("memberUid") {
			if uid == "" {
				continue
			}

			membershipID := fmt.Sprintf("%s_%s", groupCN, uid)
			obs := Observation{
				EntityType: "ops_group_member",
				EntityID:   membershipID,
				StateKey:   "ldap_group_member",
				Value:      fmt.Sprintf("%s in %s", uid, groupCN),
				DataJSON: map[string]interface{}{
					"group_name":  groupCN,
					"user_uid":    uid,
					"source":      "ldap",
					"ldap_server": config.BaseURL,
				},
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

// connectLDAP establishes a connection to the LDAP server with TLS and binds.
func connectLDAP(config *ImportConfig) (*ldap.Conn, error) {
	serverURL := config.BaseURL
	if serverURL == "" {
		return nil, fmt.Errorf("ldap base_url not configured")
	}

	var conn *ldap.Conn
	var err error

	retryConfig := runner.RetryConfig{
		MaxAttempts:  config.MaxRetries,
		BaseDelay:    time.Second,
		Multiplier:   2.0,
		JitterFrac:   0.25,
		MaxTotalTime: 15 * time.Second,
	}

	connectErr := runner.WithRetry(retryConfig, func() error {
		if strings.HasPrefix(serverURL, "ldaps://") {
			conn, err = ldap.DialURL(serverURL, ldap.DialWithTLSConfig(&tls.Config{
				MinVersion: tls.VersionTLS12,
			}))
		} else {
			conn, err = ldap.DialURL(serverURL)
			if err == nil {
				// upgrade to TLS via StartTLS
				startTLSErr := conn.StartTLS(&tls.Config{
					MinVersion: tls.VersionTLS12,
				})
				if startTLSErr != nil {
					conn.Close()
					return fmt.Errorf("starttls: %w", startTLSErr)
				}
			}
		}
		return err
	})
	if connectErr != nil {
		return nil, fmt.Errorf("connecting to %s: %w", serverURL, connectErr)
	}

	// bind with credentials
	bindDN := config.LDAPBindDN
	bindPass := config.LDAPBindPass
	if bindDN != "" && bindPass != "" {
		if err := conn.Bind(bindDN, bindPass); err != nil {
			conn.Close()
			return nil, fmt.Errorf("ldap bind as %s: %w", bindDN, err)
		}
	}

	return conn, nil
}

// extractCNFromDN extracts the CN value from a distinguished name.
// "cn=Admin Group,ou=groups,dc=example,dc=com" → "Admin Group"
func extractCNFromDN(dn string) string {
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "cn=") {
			return part[3:]
		}
	}
	return ""
}

// extractUIDFromDN extracts the uid value from a distinguished name.
// "uid=jdoe,ou=people,dc=example,dc=com" → "jdoe"
func extractUIDFromDN(dn string) string {
	parts := strings.Split(dn, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(part), "uid=") {
			return part[4:]
		}
	}
	return ""
}

// isLDAPAccountActive determines if an LDAP account is active by checking
// common account status attributes.
func isLDAPAccountActive(entry *ldap.Entry) bool {
	// check accountStatus (common in many LDAP schemas)
	status := strings.ToLower(entry.GetAttributeValue("accountStatus"))
	if status == "disabled" || status == "inactive" || status == "locked" {
		return false
	}

	// check userAccountControl (Active Directory via LDAP)
	uac := entry.GetAttributeValue("userAccountControl")
	if uac != "" {
		// bit 1 (0x0002) = ACCOUNTDISABLE
		var uacVal int
		if _, err := fmt.Sscanf(uac, "%d", &uacVal); err == nil {
			if uacVal&0x0002 != 0 {
				return false
			}
		}
	}

	return true
}

// isExcludedGroup checks if a group name is in the exclusion list.
func isExcludedGroup(name string, excludes []string) bool {
	for _, ex := range excludes {
		if strings.EqualFold(name, ex) {
			return true
		}
	}
	return false
}
