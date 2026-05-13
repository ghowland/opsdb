// === importers/opsdb-import-identity/cmd/main.go ===
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	identity "github.com/ghowland/opsdb/tools/importers/opsdb-import-identity"
	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

type CycleSummary struct {
	UsersWritten       int
	GroupsWritten      int
	MembershipsWritten int
	Errors             []string
}

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()
	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("opsdb-import-identity", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)
	logger.Info("identity importer starting", runner.Field{Key: "dos_path", Value: *dosPath})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", runner.Field{Key: "signal", Value: sig.String()})
		runner.RequestShutdown(config)
	}()

	for runner.ShouldRun(config) {
		runner.RefreshConfig(config)
		jobID, err := runner.StartCycle(config)
		if err != nil {
			logger.Error("failed to start cycle", runner.Field{Key: "error", Value: err.Error()})
			runner.WaitForNextCycle(config)
			continue
		}
		cycleLogger := logger.WithJobID(jobID)

		summary, err := runCycle(config, cycleLogger)
		if err != nil {
			cycleLogger.Error("cycle failed", runner.Field{Key: "error", Value: err.Error()})
			runner.FinishCycle(config, jobID, "failed", summary)
		} else {
			cycleLogger.Info("cycle completed",
				runner.Field{Key: "users_written", Value: summary.UsersWritten},
				runner.Field{Key: "groups_written", Value: summary.GroupsWritten},
				runner.Field{Key: "memberships_written", Value: summary.MembershipsWritten},
				runner.Field{Key: "error_count", Value: len(summary.Errors)},
			)
			runner.FinishCycle(config, jobID, "succeeded", summary)
		}
		runner.WaitForNextCycle(config)
	}

	logger.Info("identity importer stopped")
	os.Exit(0)
}

func runCycle(config *runner.RunnerConfig, logger *runner.Logger) (*CycleSummary, error) {
	summary := &CycleSummary{}
	client := config.APIClient
	dryRun := runner.IsDryRun(config)

	// GET phase: determine backend and build import config
	backend := config.SpecData.StringOrDefault("backend", "")
	if backend == "" {
		return summary, fmt.Errorf("get phase failed: backend not configured in runner_spec (expected okta, azuread, or ldap)")
	}

	logger.Info("get phase complete",
		runner.Field{Key: "backend", Value: backend},
	)

	importConfig := &identity.ImportConfig{
		Backend:    backend,
		BaseURL:    config.SpecData.StringOrDefault("base_url", ""),
		APIToken:   config.ResolveCredential("api_token"),
		BatchSize:  config.SpecData.IntOrDefault("batch_size", 200),
		MaxRetries: config.SpecData.IntOrDefault("max_retries", 3),
		Domain:     config.SpecData.StringOrDefault("domain", ""),
		TenantID:   config.SpecData.StringOrDefault("tenant_id", ""),
		ClientID:   config.ResolveCredential("client_id"),
		ClientSecret: config.ResolveCredential("client_secret"),
		LDAPBaseDN:   config.SpecData.StringOrDefault("ldap_base_dn", ""),
		LDAPBindDN:   config.SpecData.StringOrDefault("ldap_bind_dn", ""),
		LDAPBindPass: config.ResolveCredential("ldap_bind_password"),
		UserFilter:   config.SpecData.StringOrDefault("user_filter", ""),
		GroupFilter:  config.SpecData.StringOrDefault("group_filter", ""),
		ExcludeGroups: config.SpecData.StringListOrDefault("exclude_groups", nil),
	}

	// ACT phase: call the appropriate backend importer
	var users []identity.Observation
	var groups []identity.Observation
	var memberships []identity.Observation
	var err error

	switch backend {
	case "okta":
		users, err = identity.ImportOktaUsers(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: okta users: %w", err)
		}
		groups, err = identity.ImportOktaGroups(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: okta groups: %w", err)
		}
		memberships, err = identity.ImportOktaGroupMemberships(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: okta memberships: %w", err)
		}

	case "azuread":
		users, err = identity.ImportAzureADUsers(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: azuread users: %w", err)
		}
		groups, err = identity.ImportAzureADGroups(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: azuread groups: %w", err)
		}
		memberships, err = identity.ImportAzureADGroupMemberships(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: azuread memberships: %w", err)
		}

	case "ldap":
		users, err = identity.ImportLDAPUsers(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: ldap users: %w", err)
		}
		groups, err = identity.ImportLDAPGroups(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: ldap groups: %w", err)
		}
		memberships, err = identity.ImportLDAPGroupMemberships(importConfig)
		if err != nil {
			return summary, fmt.Errorf("act phase failed: ldap memberships: %w", err)
		}

	default:
		return summary, fmt.Errorf("act phase failed: unsupported backend %q (expected okta, azuread, or ldap)", backend)
	}

	if dryRun {
		logger.Info("dry run: would write observations",
			runner.Field{Key: "users", Value: len(users)},
			runner.Field{Key: "groups", Value: len(groups)},
			runner.Field{Key: "memberships", Value: len(memberships)},
		)
		summary.UsersWritten = len(users)
		summary.GroupsWritten = len(groups)
		summary.MembershipsWritten = len(memberships)
		return summary, nil
	}

	// SET phase: write all observations to OpsDB via API
	type observationSet struct {
		name    string
		obs     []identity.Observation
		counter *int
	}

	sets := []observationSet{
		{"user", users, &summary.UsersWritten},
		{"group", groups, &summary.GroupsWritten},
		{"membership", memberships, &summary.MembershipsWritten},
	}

	for _, set := range sets {
		for _, obs := range set.obs {
			writeErr := client.WriteObservation(obs.EntityType, obs.EntityID, obs.StateKey, obs.Value, obs.DataJSON)
			if writeErr != nil {
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s %s: %v", set.name, obs.EntityID, writeErr))
				logger.Warn("failed to write observation",
					runner.Field{Key: "type", Value: set.name},
					runner.Field{Key: "entity_id", Value: obs.EntityID},
					runner.Field{Key: "error", Value: writeErr.Error()},
				)
				continue
			}
			*set.counter++
		}
	}

	_ = strings.TrimSpace // suppress unused import if needed

	return summary, nil
}