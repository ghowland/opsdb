//# tools/runners/notification-runner/backends/webhook.go

go
package backends

import (
	"fmt"
)

// WebhookBackend delivers notifications via HTTP POST to a configured URL.
type WebhookBackend struct {
	URL            string
	Headers        map[string]string // additional headers (auth tokens, content-type overrides)
	TimeoutSeconds int
	// TODO: http.Client with configured timeout and TLS
}

// WebhookConfig holds webhook configuration loaded from runner spec.
type WebhookConfig struct {
	URL            string
	Headers        map[string]string
	AuthTokenEnv   string // environment variable for auth token header
	TimeoutSeconds int
}

// NewWebhookBackend creates a webhook backend from configuration.
func NewWebhookBackend(config *WebhookConfig) (*WebhookBackend, error) {
	// TODO: validate URL is non-empty and parseable
	// TODO: if config.AuthTokenEnv set:
	//   read token from environment
	//   add "Authorization: Bearer {token}" to headers
	// TODO: create http.Client with timeout from config (default 30s)
	// TODO: return configured WebhookBackend
	return nil, fmt.Errorf("not implemented")
}

// Send delivers one notification as a JSON POST to the configured URL.
func (b *WebhookBackend) Send(msg interface{}) (interface{}, error) {
	// TODO: build JSON payload from msg:
	//   {
	//     "trigger_type": msg.Trigger.TriggerType,
	//     "entity_type": msg.Trigger.EntityType,
	//     "entity_id": msg.Trigger.EntityID,
	//     "severity": msg.Trigger.Severity,
	//     "title": msg.Subject,
	//     "body": msg.Body,
	//     "recipient": msg.Recipient.Username,
	//     "metadata": msg.Metadata,
	//     "timestamp": now
	//   }
	// TODO: create POST request to b.URL
	// TODO: set Content-Type: application/json
	// TODO: set additional headers from b.Headers
	// TODO: send request
	// TODO: if response status 2xx: return DeliveryResult{Success: true, MessageID: response header or body ID}
	// TODO: if response status != 2xx: return DeliveryResult{Success: false, ErrorMsg: status + body}
	// TODO: if network error: return DeliveryResult{Success: false, ErrorMsg: err}
	return nil, nil
}

// Type returns "webhook".
func (b *WebhookBackend) Type() string {
	return "webhook"
}

// Healthy tests webhook endpoint by sending a HEAD request.
func (b *WebhookBackend) Healthy() bool {
	// TODO: send HEAD request to b.URL with short timeout (5s)
	// TODO: if status 2xx or 405 (method not allowed but server is up): return true
	// TODO: if error or other status: return false
	return false
}


