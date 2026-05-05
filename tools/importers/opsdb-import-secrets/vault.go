
// === importers/opsdb-import-secrets/vault.go ===
package secrets

// SecretMetadataObservation is the observation structure for secret metadata importers.
type SecretMetadataObservation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportVaultMetadata reads secret paths and metadata from HashiCorp Vault.
// NEVER reads secret values. Only metadata: paths, versions, creation times,
// rotation timestamps, expiration dates.
func ImportVaultMetadata(config *SecretImportConfig) ([]SecretMetadataObservation, error) {
	// TODO: list secret engine mounts
	// TODO: for each mount:
	//   list secrets recursively (list operations, not read)
	//   for each secret path:
	//     read metadata (version count, creation time, updated time, custom metadata)
	//     DO NOT read secret data endpoint
	//     create authority_pointer observation with pointer_type=secret
	//     extract rotation metadata from custom metadata if present
	//     create rotation status observation for compliance tracking
	return nil, nil
}

// SecretImportConfig holds secret metadata importer configuration.
type SecretImportConfig struct {
	BackendType    string // vault, aws_sm
	MountPaths     []string // which secret engine mounts to scan; empty = all
	RecursiveList  bool
	BatchSize      int
	MaxRetries     int
}


