package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: opsdb-import-gcp --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath

	config, err := runner.Init("opsdb-import-gcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	for runner.ShouldRun(config) {
		jobID, err := runner.StartCycle(config)
		if err != nil {
			config.Logger.Error("failed to start cycle", runner.Field("error", err.Error()))
			continue
		}
		client := config.Client.WithCorrelation(jobID, "")

		// --- GET ---
		projects, _ := runner.GetSpecDataStringSlice(config, "projects")
		if len(projects) == 0 {
			config.Logger.Error("no GCP projects configured in runner spec")
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": "no projects configured",
			})
			runner.WaitForNextCycle(config)
			continue
		}
		resourceTypes, _ := runner.GetSpecDataStringSlice(config, "resource_types")
		if len(resourceTypes) == 0 {
			resourceTypes = []string{"gce", "cloudsql", "gcs", "gke", "iam"}
		}
		batchSize, _ := runner.GetSpecDataInt(config, "batch_size")
		if batchSize == 0 {
			batchSize = 500
		}
		maxDuration, _ := runner.GetSpecDataInt(config, "max_cycle_duration_seconds")
		if maxDuration == 0 {
			maxDuration = 120
		}

		importConfig := &GCPImportConfig{
			Projects:      projects,
			ResourceTypes: resourceTypes,
			BatchSize:     batchSize,
		}

		config.Logger.Info("starting GCP import cycle",
			runner.Field("projects", projects),
			runner.Field("resource_types", resourceTypes),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			runner.LogPlan(config.Logger, "GCP import", map[string]interface{}{
				"projects":       projects,
				"resource_types": resourceTypes,
				"batch_size":     batchSize,
			})
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{"dry_run": true})
			runner.WaitForNextCycle(config)
			continue
		}

		var allObservations []Observation
		resourceTypeCounts := make(map[string]int)
		var errors []string
		cycleStart := time.Now()

		for _, rt := range resourceTypes {
			if time.Since(cycleStart).Seconds() > float64(maxDuration) {
				config.Logger.Warn("max cycle duration reached, stopping imports")
				runner.RecordBoundHit(config, "max_cycle_duration", maxDuration)
				break
			}

			var obs []Observation
			var importErr error

			switch rt {
			case "gce":
				obs, importErr = ImportGCE(importConfig)
			case "cloudsql":
				obs, importErr = ImportCloudSQL(importConfig)
			case "gcs":
				obs, importErr = ImportGCS(importConfig)
			case "gke":
				obs, importErr = ImportGKE(importConfig)
			case "iam":
				obs, importErr = ImportIAM(importConfig)
			default:
				config.Logger.Warn("unknown resource type, skipping",
					runner.Field("resource_type", rt))
				continue
			}

			if importErr != nil {
				config.Logger.Error("import failed for resource type",
					runner.Field("resource_type", rt),
					runner.Field("error", importErr.Error()),
				)
				errors = append(errors, fmt.Sprintf("%s: %v", rt, importErr))
				continue
			}

			config.Logger.Info("imported resource type",
				runner.Field("resource_type", rt),
				runner.Field("observation_count", len(obs)),
			)
			resourceTypeCounts[rt] = len(obs)
			allObservations = append(allObservations, obs...)

			if len(allObservations) >= batchSize {
				config.Logger.Warn("batch size reached, stopping imports")
				runner.RecordBoundHit(config, "batch_size", batchSize)
				break
			}
		}

		// --- SET ---
		written := 0
		writeErrors := 0
		for _, obs := range allObservations {
			_, err := client.WriteObservation(&runner.WriteObservationParams{
				TargetTable:  "observation_cache_state",
				Key:          obs.StateKey,
				Value:        obs.Value,
				DataJSON:     obs.DataJSON,
				RunnerJobID:  jobID,
				AuthorityID:  obs.AuthorityID,
				ObservedTime: time.Now(),
			})
			if err != nil {
				config.Logger.Error("failed to write observation",
					runner.Field("entity_type", obs.EntityType),
					runner.Field("entity_id", obs.EntityID),
					runner.Field("error", err.Error()),
				)
				writeErrors++
				continue
			}
			written++
		}

		status := "completed"
		if len(errors) > 0 || writeErrors > 0 {
			status = "completed_with_errors"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"observations_collected": len(allObservations),
			"observations_written":   written,
			"write_errors":           writeErrors,
			"resource_type_counts":   resourceTypeCounts,
			"errors":                 errors,
		})

		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("GCP importer shutting down")
	os.Exit(0)
}

// GCPImportConfig holds GCP importer cycle configuration.
type GCPImportConfig struct {
	Projects      []string
	ResourceTypes []string
	BatchSize     int
}

// Observation is the GCP importer observation structure.
type Observation struct {
	EntityType  string
	EntityID    string
	StateKey    string
	Value       string
	DataJSON    map[string]interface{}
	AuthorityID int
}
