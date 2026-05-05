package backends

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	notification "github.com/ghowland/opsdb/tools/runners/notification-runner"
)

// EmailBackend delivers notifications via SMTP.
type EmailBackend struct {
	SMTPHost    string
	SMTPPort    int
	Username    string
	Password    string
	FromAddress string
	UseTLS      bool
}

// EmailConfig holds SMTP configuration loaded from runner spec.
type EmailConfig struct {
	SMTPHost    string
	SMTPPort    int
	Username    string
	PasswordEnv string
	FromAddress string
	UseTLS      bool
}

// NewEmailBackend creates an email backend from configuration.
func NewEmailBackend(config *EmailConfig) (*EmailBackend, error) {
	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is empty")
	}
	if config.SMTPPort == 0 {
		return nil, fmt.Errorf("SMTP port is zero")
	}
	if config.FromAddress == "" {
		return nil, fmt.Errorf("from address is empty")
	}

	password := ""
	if config.PasswordEnv != "" {
		password = os.Getenv(config.PasswordEnv)
		if password == "" {
			return nil, fmt.Errorf("SMTP password environment variable %s is not set", config.PasswordEnv)
		}
	}

	return &EmailBackend{
		SMTPHost:    config.SMTPHost,
		SMTPPort:    config.SMTPPort,
		Username:    config.Username,
		Password:    password,
		FromAddress: config.FromAddress,
		UseTLS:      config.UseTLS,
	}, nil
}

// Send delivers one notification via SMTP email.
func (b *EmailBackend) Send(msg *notification.NotificationMessage) (*notification.DeliveryResult, error) {
	result := &notification.DeliveryResult{
		Backend:   "email",
		Recipient: msg.Recipient.Email,
		SentTime:  time.Now(),
	}

	if msg.Recipient.Email == "" {
		result.Success = false
		result.ErrorMsg = "recipient has no email address"
		return result, fmt.Errorf("recipient %s has no email address", msg.Recipient.Username)
	}

	// Build email message.
	messageID := uuid.New().String() + "@opsdb"
	result.MessageID = messageID

	headers := map[string]string{
		"From":         b.FromAddress,
		"To":           msg.Recipient.Email,
		"Subject":      msg.Subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=utf-8",
		"Message-ID":   "<" + messageID + ">",
		"Date":         time.Now().UTC().Format(time.RFC1123Z),
		"X-OpsDB-Trigger-Type": msg.Trigger.TriggerType,
	}

	var emailBody strings.Builder
	for key, value := range headers {
		emailBody.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}
	emailBody.WriteString("\r\n")
	emailBody.WriteString(msg.Body)

	addr := fmt.Sprintf("%s:%d", b.SMTPHost, b.SMTPPort)

	var sendErr error

	if b.UseTLS {
		sendErr = b.sendWithTLS(addr, msg.Recipient.Email, emailBody.String())
	} else {
		sendErr = b.sendWithSTARTTLS(addr, msg.Recipient.Email, emailBody.String())
	}

	if sendErr != nil {
		result.Success = false
		result.ErrorMsg = sendErr.Error()
		return result, sendErr
	}

	result.Success = true
	return result, nil
}

// sendWithTLS connects to the SMTP server over TLS (implicit TLS, typically port 465).
func (b *EmailBackend) sendWithTLS(addr string, to string, message string) error {
	tlsConfig := &tls.Config{
		ServerName: b.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial to %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, b.SMTPHost)
	if err != nil {
		return fmt.Errorf("creating SMTP client: %w", err)
	}
	defer client.Close()

	return b.sendViaClient(client, to, message)
}

// sendWithSTARTTLS connects to the SMTP server in plain text and upgrades
// to TLS via STARTTLS if the server supports it (typically port 587).
func (b *EmailBackend) sendWithSTARTTLS(addr string, to string, message string) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dial to %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, b.SMTPHost)
	if err != nil {
		return fmt.Errorf("creating SMTP client: %w", err)
	}
	defer client.Close()

	// Attempt STARTTLS if server advertises it.
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: b.SMTPHost,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS: %w", err)
		}
	}

	return b.sendViaClient(client, to, message)
}

// sendViaClient authenticates and sends the message through an established SMTP client.
func (b *EmailBackend) sendViaClient(client *smtp.Client, to string, message string) error {
	// Authenticate if credentials are provided.
	if b.Username != "" && b.Password != "" {
		auth := smtp.PlainAuth("", b.Username, b.Password, b.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth: %w", err)
		}
	}

	if err := client.Mail(b.FromAddress); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}

	_, err = writer.Write([]byte(message))
	if err != nil {
		writer.Close()
		return fmt.Errorf("writing message body: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing message: %w", err)
	}

	if err := client.Quit(); err != nil {
		// Quit failure after successful send is non-fatal.
		_ = err
	}

	return nil
}

// Type returns "email".
func (b *EmailBackend) Type() string {
	return "email"
}

// Healthy tests SMTP connectivity by dialing the server with a short timeout.
func (b *EmailBackend) Healthy() bool {
	addr := fmt.Sprintf("%s:%d", b.SMTPHost, b.SMTPPort)

	if b.UseTLS {
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 5 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: b.SMTPHost, MinVersion: tls.VersionTLS12})
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
