# Writing an Importer

How to write a new OpsDB importer that pulls data from an external authority.

## What an importer is

An importer is a runner of kind **puller**. It reads from an external authority
(AWS, GCP, K8s, Okta, Prometheus, PagerDuty, Vault, etc.), transforms the data
to OpsDB schema shape, and writes observations via the API. It never modifies
the authority — it only reads.

## Structure

Every importer follows this file layout:

```
tools/importers/opsdb-import-{authority}/
├── cmd/
│   └── main.go          # CLI entrypoint, runner lifecycle loop
├── mapping.go           # Authority data → OpsDB entity mapping
├── {resource_type}.go   # One file per resource type
└── ...
```

## Skeleton

### cmd/main.go

```go
package main

import (
    "os"
    runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

func main() {
    config, err := runner.Init("opsdb-import-myauthority")
    if err != nil { os.Exit(1) }

    for runner.ShouldRun(config) {
        jobID, _ := runner.StartCycle(config)
        client := config.Client.WithCorrelation(jobID, "")

        // GET: read runner spec for config (resource types, credentials source)
        resourceTypes, _ := runner.GetSpecDataStringSlice(config, "resource_types")

        // ACT: call each resource importer, collect observations
        var observations []Observation
        for _, rt := range resourceTypes {
            obs, err := importResourceType(rt, config)
            if err != nil { config.Logger.Error("import failed", runner.Field("type", rt)) }
            observations = append(observations, obs...)
        }

        // SET: write observations
        if !runner.IsDryRun(config) {
            for _, obs := range observations {
                client.WriteObservation(&runner.WriteObservationParams{
                    TargetTable: "observation_cache_state",
                    Key:         obs.StateKey,
                    Value:       obs.Value,
                    DataJSON:    obs.DataJSON,
                    RunnerJobID: jobID,
                    AuthorityID: authorityID,
                    ObservedTime: time.Now(),
                })
            }
        }

        runner.FinishCycle(config, "completed", summary)
        runner.WaitForNextCycle(config)
    }
}
```

### Per-resource file

Each resource type file has one public function that reads from the authority
and returns observations:

```go
func ImportWidgets(config *ImportConfig) ([]Observation, error) {
    // Call authority API (paginate, handle rate limits)
    // For each resource: call MapWidget(raw) to transform
    // Return observations
}
```

### mapping.go

The mapping file contains the DSNC (flattening) decisions. This is where you
decide what goes in the JSON payload versus what breaks out into separate entities.

**Flatten when:** the nested data is per-row metadata of the parent. Instance type,
AMI ID, VPC ID for an EC2 instance → flat fields in `cloud_data_json`.

**Break out when:** the nested data has independent lifecycle, its own identity,
or appears as a list of N items. Security group memberships → bridge table.
Attached EBS volumes → separate cloud_resource rows.

**The list-of-N test:** if you find yourself writing `volumes_0_id`, `volumes_1_id`,
stop. That is a list with variable length and positional indices. It needs a
sub-table or separate entity, not flat payload fields.

```go
func MapEC2Instance(raw *ec2.Instance) Observation {
    return Observation{
        EntityType: "cloud_resource",
        EntityID:   *raw.InstanceId,
        StateKey:   "ec2_instance",
        DataJSON: map[string]interface{}{
            "instance_type":    *raw.InstanceType,
            "ami_id":           *raw.ImageId,
            "vpc_id":           *raw.VpcId,
            "subnet_id":        *raw.SubnetId,
            "private_ip":       *raw.PrivateIpAddress,
            "state":            *raw.State.Name,
            "launch_time":      raw.LaunchTime.Format(time.RFC3339),
            // NOT flattened: security groups, volumes, tags (these are N-of)
        },
    }
}
```

## Credential handling

Never store credentials in code or config files. Reference environment variables:

```yaml
# dos/prod-0/importers/credentials/myauthority.yaml
myauthority:
  api_token_env_var: OPSDB_MYAUTHORITY_TOKEN
  api_endpoint_env_var: OPSDB_MYAUTHORITY_ENDPOINT
```

The importer reads the env var at runtime. The secret backend issues the token.
The OpsDB stores a pointer to the secret path, never the value.

## Report keys

Before the importer can write observations, its runner spec must declare
report keys for every key it writes. The API rejects undeclared keys (fail-closed).

Report keys are declared in `runner_report_key` rows linked to the runner spec.
They specify which `target_table` and which `report_key` values the runner is
allowed to write. Create these via change set when registering the importer.

## Observation targets

Most importers write to `observation_cache_state`, keyed by:
- `entity_type` + `entity_id` + `state_key`

Some write to `observation_cache_metric` for numeric data, keyed by:
- `authority_id` + `hostname` + `metric_key`

Some write to `observation_cache_config` for configuration data, keyed by:
- `authority_id` + `hostname` + `config_key`

All observation cache writes set `_observed_time` (when the observation was taken)
and `_puller_runner_job_id` (which job wrote it). These fields are used by
freshness checks and audit trails.

## Watch-based importers

For authorities that support watch/subscribe (Kubernetes, etcd), implement
the level-triggered backstop pattern:

1. On connect: full list of all resources (snapshot).
2. Stream incremental changes via watch.
3. On disconnect: reconnect, full re-list, then resume streaming.

This ensures missed events during disconnection are caught by the re-list.
See `tools/importers/opsdb_import_k8s/watcher.go` for the reference implementation.
