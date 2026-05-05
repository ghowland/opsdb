//# tools/runners/notification-runner/backends/email.go

go
package backends

import (
	"fmt"
)

// EmailBackend delivers notifications via SMTP.
type EmailBackend struct {
	SMTPHost     string
	SMTPPort     int
	Username     string
	Password     string // resolved from secret backend, never persisted
	FromAddress  string
	UseTLS       bool
	// TODO: smtp connection pool or dialer
}

// EmailConfig holds SMTP configuration loaded from runner spec.
type EmailConfig struct {
	SMTPHost    string
	SMTPPort    int
	Username    string
	PasswordEnv string // environment variable name holding SMTP password
	FromAddress string
	UseTLS      bool
}

// NewEmailBackend creates an email backend from configuration.
func NewEmailBackend(config *EmailConfig) (*EmailBackend, error) {
	// TODO: read password from environment variable config.PasswordEnv
	//   if empty: return error "SMTP password environment variable not set"
	// TODO: validate host and port are set
	// TODO: validate from address is non-empty
	// TODO: return configured EmailBackend
	return nil, fmt.Errorf("not implemented")
}

// Send delivers one notification via SMTP email.
func (b *EmailBackend) Send(msg interface{}) (interface{}, error) {
	// TODO: extract recipient email, subject, body from msg
	// TODO: build email:
	//   From: b.FromAddress
	//   To: recipient email
	//   Subject: msg subject
	//   Content-Type: text/plain; charset=utf-8
	//   Body: msg body
	// TODO: connect to SMTP server:
	//   if b.UseTLS: dial with TLS
	//   else: dial plain, STARTTLS if server supports
	// TODO: authenticate with b.Username, b.Password
	// TODO: send email
	// TODO: on success: return DeliveryResult{Success: true, MessageID: server-assigned-id}
	// TODO: on failure: return DeliveryResult{Success: false, ErrorMsg: err.Error()}
	return nil, nil
}

// Type returns "email".
func (b *EmailBackend) Type() string {
	return "email"
}

// Healthy tests SMTP connectivity by dialing the server.
func (b *EmailBackend) Healthy() bool {
	// TODO: dial SMTP server with short timeout (5s)
	// TODO: if connection succeeds: close, return true
	// TODO: if error: return false
	return false
}


