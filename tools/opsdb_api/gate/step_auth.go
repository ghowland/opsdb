//# tools/opsdb_api/gate/step_auth.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/tools/opsdb_api/auth"
)

// stepAuthenticate is gate step 1: Authentication.
// Validates caller credentials via the configured auth provider and
// resolves to an Identity. The identity carries ops_user_id (human),
// runner_machine_id (runner), or both (web-application-mediated write
// where a runner acts on behalf of a human).
//
// Rejects if: no auth provider configured, no credentials provided,
// authentication fails, identity is nil, or identity has no resolved
// principal (neither ops_user_id nor runner_machine_id).
func stepAuthenticate(ctx *GateContext) {
	if ctx.AuthProvider == nil {
		reject(ctx, 1, "auth_not_configured",
			"no authentication provider is configured", nil)
		return
	}

	creds := ctx.Request.RawCredentials

	if isCredentialsEmpty(creds) {
		reject(ctx, 1, "authentication_failed",
			"no credentials provided", map[string]interface{}{
				"client_ip":  ctx.Request.ClientIP,
				"user_agent": ctx.Request.UserAgent,
			})
		return
	}

	identity, err := ctx.AuthProvider.Authenticate(creds)
	if err != nil {
		reject(ctx, 1, "authentication_failed",
			fmt.Sprintf("authentication failed: %v", err),
			map[string]interface{}{
				"auth_method": ctx.AuthProvider.Type(),
				"client_ip":   ctx.Request.ClientIP,
				"user_agent":  ctx.Request.UserAgent,
			})
		return
	}

	if identity == nil {
		reject(ctx, 1, "authentication_failed",
			"auth provider returned nil identity", nil)
		return
	}

	// The identity must have at least one resolved principal. A human
	// has OpsUserID set. A runner has RunnerMachineID set. A web-mediated
	// write has both. An identity with neither means the auth provider
	// matched credentials but couldn't map them to an operational identity
	// in the OpsDB — the user or service account exists in the IdP but
	// has no corresponding ops_user or runner_machine row.
	if !identity.IsHuman() && !identity.IsRunner() {
		reject(ctx, 1, "authentication_failed",
			"identity resolved but has no ops_user_id or runner_machine_id",
			map[string]interface{}{
				"username":    identity.Username,
				"auth_method": identity.AuthMethod,
			})
		return
	}

	ctx.Identity = identity
}

// isCredentialsEmpty returns true if the credentials struct contains no
// authentication material at all — no basic auth, no bearer token. This
// is checked before calling the auth provider so we can give a clear
// "no credentials provided" error rather than a provider-specific error
// about missing fields.
//
// When OIDC and service account providers are added, this function
// extends to check their credential fields as well.
func isCredentialsEmpty(creds auth.Credentials) bool {
	return !creds.HasBasicAuth() && !creds.HasBearerToken()
}
