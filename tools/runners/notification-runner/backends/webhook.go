package backends

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	notification "github.com/ghowland/opsdb/tools/runners/notification-runner"
)

// WebhookBackend delivers notifications via HTTP POST to a configured URL.
type WebhookBackend struct {
	URL        string
	Headers    map[string]string
	httpClient *http.Client
}

// WebhookConfig holds webhook configuration loaded from runner spec.
type WebhookConfig struct {
	URL            string
	Headers        map[string]string
	AuthTokenEnv   string
	TimeoutSeconds int
}

// NewWebhookBackend creates a webhook backend from configuration.
func NewWebhookBackend(config *WebhookConfig) (*WebhookBackend, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("webhook URL is empty")
	}
	if _, err := url.Parse(config.URL); err != nil {
		return nil, fmt.Errorf("webhook URL is not valid: %w", err)
	}

	headers := make(map[string]string)
	if config.Headers != nil {
		for k, v := range config.Headers {
			headers[k] = v
		}
	}

	// Read auth token from environment if configured.
	if config.AuthTokenEnv != "" {
		token := os.Getenv(config.AuthTokenEnv)
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	timeoutSec := config.TimeoutSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		MaxIdleConns:          5,
		IdleConnTimeout:       60 * time.Second,
		ResponseHeaderTimeout: time.Duration(timeoutSec) * time.Second,
	}

	return &WebhookBackend{
		URL:     config.URL,
		Headers: headers,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   time.Duration(timeoutSec) * time.Second,
		},
	}, nil
}

// Send delivers one notification as a JSON POST to the configured URL.
func (b *WebhookBackend) Send(msg *notification.NotificationMessage) (*notification.DeliveryResult, error) {
	result := &notification.DeliveryResult{
		Backend:   "webhook",
		Recipient: msg.Recipient.Username,
		SentTime:  time.Now(),
		MessageID: uuid.New().String(),
	}

	payload := map[string]interface{}{
		"trigger_type": msg.Trigger.TriggerType,
		"entity_type":  msg.Trigger.EntityType,
		"entity_id":    msg.Trigger.EntityID,
		"severity":     msg.Trigger.Severity,
		"title":        msg.Subject,
		"body":         msg.Body,
		"recipient":    msg.Recipient.Username,
		"metadata":     msg.Metadata,
		"timestamp":    time.Now().UTC().Format(time.RFC3339Nano),
		"message_id":   result.MessageID,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("marshaling payload: %v", err)
		return result, err
	}

	req, err := http.NewRequest("POST", b.URL, bytes.NewReader(jsonBytes))
	if err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("creating request: %v", err)
		return result, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpsDB-Message-ID", result.MessageID)
	req.Header.Set("X-OpsDB-Trigger-Type", msg.Trigger.TriggerType)

	for key, value := range b.Headers {
		req.Header.Set(key, value)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		result.Success = false
		result.ErrorMsg = fmt.Sprintf("request failed: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.Success = true

		// Try to extract message ID from response.
		var respData map[string]interface{}
		if json.Unmarshal(respBody, &respData) == nil {
			if id, ok := respData["id"].(string); ok {
				result.MessageID = id
			}
		}

		return result, nil
	}

	result.Success = false
	result.ErrorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateBody(respBody, 512))
	return result, fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
}

// Type returns "webhook".
func (b *WebhookBackend) Type() string {
	return "webhook"
}

// Healthy tests webhook endpoint by sending a HEAD request with a short timeout.
// Returns true if the server responds with 2xx or 405 (method not allowed
// but server is reachable).
func (b *WebhookBackend) Healthy() bool {
	healthClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	req, err := http.NewRequest("HEAD", b.URL, nil)
	if err != nil {
		return false
	}

	for key, value := range b.Headers {
		req.Header.Set(key, value)
	}

	resp, err := healthClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	// 2xx = healthy. 405 = server is up but doesn't support HEAD, still healthy.
	return resp.StatusCode < 300 || resp.StatusCode == http.StatusMethodNotAllowed
}

// truncateBody truncates a response body to maxLen bytes for error messages.
func truncateBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}
