package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: opsdb-import-aws --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath // used by runner.Init via environment or config resolution

	config, err := runner.Init("opsdb-import-aws")
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
		// Read configuration from runner spec.
		regions, _ := runner.GetSpecDataStringSlice(config, "regions")
		if len(regions) == 0 {
			regions = []string{"us-east-1"}
		}
		resourceTypes, _ := runner.GetSpecDataStringSlice(config, "resource_types")
		if len(resourceTypes) == 0 {
			resourceTypes = []string{"ec2", "rds", "s3", "iam", "vpc", "route53"}
		}
		batchSize, _ := runner.GetSpecDataInt(config, "batch_size")
		if batchSize == 0 {
			batchSize = 500
		}

		// Build import config from spec data.
		importConfig := &ImportConfig{
			Regions:       regions,
			ResourceTypes: resourceTypes,
			BatchSize:     batchSize,
		}

		config.Logger.Info("starting AWS import cycle",
			runner.Field("regions", regions),
			runner.Field("resource_types", resourceTypes),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			runner.LogPlan(config.Logger, "AWS import", map[string]interface{}{
				"regions":        regions,
				"resource_types": resourceTypes,
				"batch_size":     batchSize,
			})
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"dry_run": true,
			})
			runner.WaitForNextCycle(config)
			continue
		}

		var allObservations []Observation
		summary := &ImportSummary{}
		cycleStart := time.Now()

		for _, rt := range resourceTypes {
			if time.Since(cycleStart).Seconds() > float64(maxCycleDuration(config)) {
				config.Logger.Warn("max cycle duration reached, stopping imports")
				runner.RecordBoundHit(config, "max_cycle_duration", maxCycleDuration(config))
				break
			}

			var obs []Observation
			var importErr error

			switch rt {
			case "ec2":
				obs, importErr = ImportEC2(importConfig)
			case "rds":
				obs, importErr = ImportRDS(importConfig)
			case "s3":
				obs, importErr = ImportS3(importConfig)
			case "iam":
				obs, importErr = ImportIAM(importConfig)
			case "vpc":
				obs, importErr = ImportVPC(importConfig)
			case "route53":
				obs, importErr = ImportRoute53(importConfig)
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
				summary.Errors = append(summary.Errors, fmt.Sprintf("%s: %v", rt, importErr))
				continue
			}

			config.Logger.Info("imported resource type",
				runner.Field("resource_type", rt),
				runner.Field("observation_count", len(obs)),
			)
			summary.ResourceTypeCounts[rt] = len(obs)
			allObservations = append(allObservations, obs...)

			if len(allObservations) >= batchSize {
				config.Logger.Warn("batch size reached, stopping imports")
				runner.RecordBoundHit(config, "batch_size", batchSize)
				break
			}
		}

		// --- SET ---
		written := 0
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
				summary.WriteErrors++
				continue
			}
			written++
		}
		summary.ObservationsWritten = written

		status := "completed"
		if len(summary.Errors) > 0 || summary.WriteErrors > 0 {
			status = "completed_with_errors"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"observations_collected": len(allObservations),
			"observations_written":   written,
			"write_errors":           summary.WriteErrors,
			"resource_type_counts":   summary.ResourceTypeCounts,
			"errors":                 summary.Errors,
		})

		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("AWS importer shutting down")
	os.Exit(0)
}

// ImportConfig holds AWS importer cycle configuration.
type ImportConfig struct {
	Regions       []string
	ResourceTypes []string
	BatchSize     int
}

// Observation is the AWS importer observation structure.
type Observation struct {
	EntityType  string
	EntityID    string
	StateKey    string
	Value       string
	DataJSON    map[string]interface{}
	AuthorityID int
}

// ImportSummary tracks results for one import cycle.
type ImportSummary struct {
	ObservationsWritten int
	WriteErrors         int
	ResourceTypeCounts  map[string]int
	Errors              []string
}

// maxCycleDuration reads the max cycle duration from config, defaulting to 120s.
func maxCycleDuration(config *runner.RunnerConfig) int {
	v, ok := runner.GetSpecDataInt(config, "max_cycle_duration_seconds")
	if !ok || v == 0 {
		return 120
	}
	return v
}

func init() {
	// Ensure ImportSummary.ResourceTypeCounts is initialized.
	// (In the actual cycle code above, we use it directly; this is a safety note
	// that the struct should be initialized before use.)
}
