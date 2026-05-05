// === importers/opsdb-import-secrets/aws_sm.go ===
package secrets

// ImportAWSSecretsManager reads secret metadata from AWS Secrets Manager.
// NEVER reads secret values. Only metadata: names, ARNs, creation dates,
// rotation configuration, last rotated time.
func ImportAWSSecretsManager(config *SecretImportConfig) ([]SecretMetadataObservation, error) {
	// TODO: paginate ListSecrets
	// TODO: for each secret:
	//   extract Name, ARN, Description, CreatedDate, LastChangedDate, LastAccessedDate
	//   extract RotationEnabled, RotationLambdaARN, RotationRules (interval)
	//   extract LastRotatedDate
	//   DO NOT call GetSecretValue
	//   create authority_pointer observation with pointer_type=secret
	//   create rotation compliance observation:
	//     was_rotated = (LastRotatedDate within RotationRules interval)
	//     days_since_rotation = now - LastRotatedDate
	return nil, nil
}
