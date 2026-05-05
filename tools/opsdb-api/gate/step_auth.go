//# tools/opsdb-api/gate/step_auth.go

package gate

import (
	"fmt"

	"github.com/ghowland/opsdb/tools/opsdb-api/auth"
)

// stepAuthenticate is gate step 1: Authentication.
// Validates caller credentials via the configured auth provider.
// Resolves to ops_user (human), runner_machine (runner), or both (web-mediated).
func stepAuthenticate(ctx *GateContext) {
	if ctx.AuthProvider == nil {
		reject(ctx, 1, "authentication_failed",
			"no auth provider configured", nil)
		return
	}

	creds := ctx.Request.RawCredentials
	if isEmpty(creds) {
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

	// verify identity has at least one resolved principal
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

// isEmpty checks whether credentials contain any authentication data.
func isEmpty(creds auth.Credentials) bool {
	return !creds.HasBasicAuth() &&
		!creds.HasBearerToken() &&
		!creds.HasOIDCToken() &&
		!creds.HasSAMLAssertion()
}
