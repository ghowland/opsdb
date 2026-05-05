//# tools/importers/opsdb-import-secrets/cmd/main.go

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
	secrets "github.com/ghowland/opsdb/tools/importers/opsdb-import-secrets"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS config directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "error: --dos flag is required\n")
		os.Exit(2)
	}

	config, err := runner.Init("opsdb-import-secrets", *dosPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to initialize opsdb-import-secrets: %v\n", err)
		os.Exit(2)
	}
	defer runner.Shutdown(config)

	logger := runner.NewLogger(config)

	backend := config.SpecData.StringOrDefault("backend", "")
	if backend == "" {
		fmt.Fprintf(os.Stderr, "error: runner spec missing required config: backend (vault or aws_sm)\n")
		os.Exit(2)
	}

	logger.Info("opsdb-import-secrets starting",
		runner.Field{Key: "dos_path", Value: *dosPath},
		runner.Field{Key: "backend", Value: backend},
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", runner.Field{Key: "signal", Value: sig.String()})
		runner.RequestShutdown(config)
	}()

	for runner.ShouldRun(config) {
		err := runner.RefreshConfig(config)
		if err != nil {
			logger.Warn("failed to refresh config, using cached", runner.Field{Key: "error", Value: err.Error()})
		}

		jobID, err := runner.StartCycle(config)
		if err != nil {
			logger.Error("failed to start cycle", runner.Field{Key: "error", Value: err.Error()})
			runner.WaitForNextCycle(config)
			continue
		}
		cycleLogger := logger.WithJobID(jobID)

		summary, err := runCycle(config, cycleLogger, backend)
		if err != nil {
			cycleLogger.Error("cycle failed", runner.Field{Key: "error", Value: err.Error()})
			runner.FinishCycle(config, jobID, "failed", summary)
		} else {
			cycleLogger.Info("cycle complete",
				runner.Field{Key: "secrets_imported", Value: summary.SecretsImported},
				runner.Field{Key: "authorities_updated", Value: summary.AuthoritiesUpdated},
				runner.Field{Key: "errors", Value: len(summary.Errors)},
			)
			status := "succeeded"
			if len(summary.Errors) > 0 {
				status = "partial"
			}
			runner.FinishCycle(config, jobID, status, summary)
		}

		runner.WaitForNextCycle(config)
	}

	logger.Info("opsdb-import-secrets stopped")
	os.Exit(0)
}

// SecretImportSummary holds the results of one import cycle.
type SecretImportSummary struct {
	SecretsImported    int
	AuthoritiesUpdated int
	Errors             []string
}

func runCycle(config *runner.RunnerConfig, logger *runner.Logger, backend string) (*SecretImportSummary, error) {
	summary := &SecretImportSummary{}
	client := config.APIClient
	dryRun := runner.IsDryRun(config)

	// GET: read authority row for the secret backend
	authorityName := config.SpecData.StringOrDefault("authority_name", "")
	if authorityName == "" {
		return summary, fmt.Errorf("runner spec missing required config: authority_name")
	}

	authorityResults, err := client.Search("authority", map[string]interface{}{
		"name":           authorityName,
		"authority_type": "secret_vault",
		"is_active":      true,
	}, nil, 1)
	if err != nil {
		return summary, fmt.Errorf("failed to look up authority %s: %w", authorityName, err)
	}
	if len(authorityResults.Rows) == 0 {
		return summary, fmt.Errorf("authority %s not found or inactive", authorityName)
	}

	authorityRow := authorityResults.Rows[0]
	authorityID, _ := authorityRow.IntField("id")

	logger.Info("importing secret metadata",
		runner.Field{Key: "authority", Value: authorityName},
		runner.Field{Key: "authority_id", Value: authorityID},
		runner.Field{Key: "backend", Value: backend},
	)

	// ACT: call the appropriate backend importer
	var observations []secrets.Observation

	switch backend {
	case "vault":
		vaultConfig := secrets.VaultConfig{
			Address:    config.SpecData.StringOrDefault("vault_addr", ""),
			TokenPath:  config.SpecData.StringOrDefault("vault_token_path", ""),
			MountPaths: config.SpecData.StringListOrDefault("mount_paths", []string{"secret"}),
			MaxDepth:   config.SpecData.IntOrDefault("max_depth", 10),
		}
		observations, err = secrets.ImportVault(&vaultConfig)
		if err != nil {
			return summary, fmt.Errorf("vault import failed: %w", err)
		}

	case "aws_sm":
		awsConfig := secrets.AWSSecretsManagerConfig{
			Region:    config.SpecData.StringOrDefault("region", ""),
			Regions:   config.SpecData.StringListOrDefault("regions", nil),
			MaxResults: config.SpecData.IntOrDefault("max_results", 1000),
		}
		observations, err = secrets.ImportAWSSecretsManager(&awsConfig)
		if err != nil {
			return summary, fmt.Errorf("aws secrets manager import failed: %w", err)
		}

	default:
		return summary, fmt.Errorf("unsupported secret backend: %s (supported: vault, aws_sm)", backend)
	}

	logger.Info("secret metadata collected",
		runner.Field{Key: "observation_count", Value: len(observations)},
	)

	if dryRun {
		for i, obs := range observations {
			if i >= 20 {
				logger.Info("dry run: truncating log output",
					runner.Field{Key: "remaining", Value: len(observations) - i},
				)
				break
			}
			logger.Info("dry run: would write observation",
				runner.Field{Key: "pointer_type", Value: "secret"},
				runner.Field{Key: "locator", Value: obs.SecretPath},
				runner.Field{Key: "last_rotated", Value: obs.LastRotatedTime},
			)
		}
		summary.SecretsImported = len(observations)
		return summary, nil
	}

	// SET: write observations as authority_pointer records
	for _, obs := range observations {
		err := writeSecretObservation(client, authorityID, obs)
		if err != nil {
			logger.Warn("failed to write secret observation",
				runner.Field{Key: "path", Value: obs.SecretPath},
				runner.Field{Key: "error", Value: err.Error()},
			)
			summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %v", obs.SecretPath, err))
			continue
		}
		summary.SecretsImported++
	}

	summary.AuthoritiesUpdated = 1
	return summary, nil
}

// writeSecretObservation writes one secret metadata observation to OpsDB
// as an authority_pointer with pointer_type=secret. Never writes secret values.
func writeSecretObservation(client *runner.APIClient, authorityID int, obs secrets.Observation) error {
	return client.WriteObservation("observation_cache_state", 0, "secret_metadata", map[string]interface{}{
		"authority_id":    authorityID,
		"entity_type":    "authority_pointer",
		"state_key":      obs.SecretPath,
		"pointer_type":   "secret",
		"locator":        obs.SecretPath,
		"pointer_data_json": map[string]interface{}{
			"secret_engine":       obs.Engine,
			"secret_version":      obs.Version,
			"last_rotated_time":   obs.LastRotatedTime,
			"rotation_enabled":    obs.RotationEnabled,
			"expiration_time":     obs.ExpirationTime,
			"tags":                obs.Tags,
		},
	})
}
