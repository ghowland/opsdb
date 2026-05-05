
// === opsdb-api/gate/step_auth.go ===
package gate

// StepAuthenticate is gate step 1: Authentication.
// Validates caller credentials via the configured auth provider.
// Resolves to ops_user (human), runner_machine (runner), or both (web-mediated).
func StepAuthenticate(ctx *GateContext) error {
	// TODO: extract credentials from ctx.Request.RawCredentials
	// TODO: call auth.Provider.Authenticate(credentials)
	// TODO: on success: set ctx.Identity with resolved user/runner IDs
	// TODO: on failure: set ctx.Rejected = true, ctx.RejectionError with step=1,
	//       code=authentication_failed, message from provider
	// TODO: log authentication attempt (success or failure) for step 8
	return nil
}

