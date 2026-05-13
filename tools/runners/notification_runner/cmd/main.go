package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
	notification "github.com/ghowland/opsdb/tools/runners/notification_runner"
	"github.com/ghowland/opsdb/tools/runners/notification_runner/backends"
)

func main() {
	dosPath := flag.String("dos", "", "path to DOS directory")
	flag.Parse()

	if *dosPath == "" {
		fmt.Fprintf(os.Stderr, "usage: notification-runner --dos <dos-directory>\n")
		os.Exit(2)
	}
	_ = dosPath

	config, err := runner.Init("notification-runner")
	if err != nil {
		fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
		os.Exit(1)
	}

	// Initialize notification backends from spec config.
	notificationBackends, err := initBackends(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backend init failed: %v\n", err)
		os.Exit(1)
	}

	// Read notification type configuration.
	enabledTypes := readEnabledTypes(config)

	// Track last cycle time for incremental query.
	lastCycleTime := time.Now().Add(-config.CycleInterval)

	for runner.ShouldRun(config) {
		jobID, err := runner.StartCycle(config)
		if err != nil {
			config.Logger.Error("failed to start cycle", runner.Field("error", err.Error()))
			runner.WaitForNextCycle(config)
			continue
		}
		client := config.Client.WithCorrelation(jobID, "")

		// --- GET ---
		batchSize, _ := runner.GetSpecDataInt(config, "batch_size")
		if batchSize == 0 {
			batchSize = 200
		}
		dedupWindowSeconds, _ := runner.GetSpecDataInt(config, "dedup_window_seconds")
		if dedupWindowSeconds == 0 {
			dedupWindowSeconds = 300
		}

		triggers, err := notification.GetNotificationTriggers(client, lastCycleTime, enabledTypes, batchSize)
		if err != nil {
			config.Logger.Error("failed to get triggers",
				runner.Field("error", err.Error()))
			runner.FinishCycle(config, "failed", map[string]interface{}{
				"error": err.Error(),
			})
			lastCycleTime = time.Now()
			runner.WaitForNextCycle(config)
			continue
		}

		config.Logger.Info("found notification triggers",
			runner.Field("trigger_count", len(triggers)),
		)

		// --- ACT ---
		if runner.IsDryRun(config) {
			planData := make([]map[string]interface{}, 0, len(triggers))
			for _, t := range triggers {
				recipients, _ := notification.ResolveRecipients(client, &t)
				planData = append(planData, map[string]interface{}{
					"trigger_type":    t.TriggerType,
					"entity_type":     t.EntityType,
					"entity_id":       t.EntityID,
					"severity":        t.Severity,
					"recipient_count": len(recipients),
				})
			}
			runner.LogPlan(config.Logger, "notifications to send", planData)
			runner.SkipActPhase(config.Logger)
			runner.SkipSetPhase(config.Logger)
			runner.FinishCycle(config, "completed", map[string]interface{}{
				"dry_run":        true,
				"triggers_found": len(triggers),
			})
			lastCycleTime = time.Now()
			runner.WaitForNextCycle(config)
			continue
		}

		notificationsSent := 0
		notificationsFailed := 0
		notificationsDeduped := 0
		backendsUsedSet := make(map[string]bool)
		var errors []string

		// Build backend lookup map.
		backendMap := make(map[string]notification.Backend)
		for _, b := range notificationBackends {
			backendMap[b.Type()] = b
		}
		var defaultBackend notification.Backend
		if len(notificationBackends) > 0 {
			defaultBackend = notificationBackends[0]
		}

		for _, trigger := range triggers {
			recipients, err := notification.ResolveRecipients(client, &trigger)
			if err != nil {
				errors = append(errors, fmt.Sprintf("resolving recipients for %s/%d: %v",
					trigger.EntityType, trigger.EntityID, err))
				continue
			}

			for _, recipient := range recipients {
				// Deduplication check.
				if notification.IsDuplicate(client, &trigger, recipient.OpsUserID, dedupWindowSeconds) {
					notificationsDeduped++
					continue
				}

				msg := notification.RenderMessage(&trigger, &recipient)

				// Select backend.
				backend := defaultBackend
				if recipient.PreferredBackend != "" {
					if b, ok := backendMap[recipient.PreferredBackend]; ok {
						backend = b
					}
				}
				if backend == nil {
					errors = append(errors, fmt.Sprintf("no backend available for %s", recipient.Username))
					notificationsFailed++
					continue
				}

				result, sendErr := backend.Send(msg)
				backendsUsedSet[backend.Type()] = true

				if sendErr != nil || (result != nil && !result.(*notification.DeliveryResult).Success) {
					notificationsFailed++
					errMsg := ""
					if sendErr != nil {
						errMsg = sendErr.Error()
					} else if result != nil {
						errMsg = result.(*notification.DeliveryResult).ErrorMsg
					}
					errors = append(errors, fmt.Sprintf("send to %s: %s", recipient.Username, errMsg))
				} else {
					notificationsSent++
				}

				// Record delivery for dedup and audit.
				deliveryResult := &notification.DeliveryResult{
					Backend:   backend.Type(),
					Recipient: recipient.Username,
					Success:   sendErr == nil,
					SentTime:  time.Now(),
				}
				if sendErr != nil {
					deliveryResult.ErrorMsg = sendErr.Error()
				}
				notification.RecordDelivery(client, deliveryResult, &trigger, recipient.OpsUserID, jobID)
			}
		}

		// --- SET ---
		backendsUsed := make([]string, 0, len(backendsUsedSet))
		for b := range backendsUsedSet {
			backendsUsed = append(backendsUsed, b)
		}

		status := "completed"
		if len(errors) > 0 {
			status = "completed_with_errors"
		}

		runner.FinishCycle(config, status, map[string]interface{}{
			"triggers_found":        len(triggers),
			"notifications_sent":    notificationsSent,
			"notifications_failed":  notificationsFailed,
			"notifications_deduped": notificationsDeduped,
			"backends_used":         backendsUsed,
			"errors":                errors,
		})

		lastCycleTime = time.Now()
		runner.WaitForNextCycle(config)
	}

	config.Logger.Info("notification runner shutting down")
	os.Exit(0)
}

// initBackends creates notification backends from the runner spec configuration.
func initBackends(config *runner.RunnerConfig) ([]notification.Backend, error) {
	backendsConfig, ok := runner.GetSpecData(config, "backends")
	if !ok {
		return nil, fmt.Errorf("no backends configured in runner spec")
	}

	backendsList, ok := backendsConfig.([]interface{})
	if !ok {
		return nil, fmt.Errorf("backends config is not a list")
	}

	var result []notification.Backend

	for i, item := range backendsList {
		backendMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("backend %d is not a map", i)
		}

		backendType, _ := backendMap["type"].(string)
		backendConfig, _ := backendMap["config"].(map[string]interface{})

		switch backendType {
		case "email":
			smtpHost := envFromConfig(backendConfig, "smtp_host_env_var")
			smtpPort := 587
			if p, ok := backendConfig["smtp_port"].(float64); ok {
				smtpPort = int(p)
			}
			username := envFromConfig(backendConfig, "username_env_var")
			password := envFromConfig(backendConfig, "password_env_var")
			fromAddress, _ := backendConfig["from_address"].(string)
			useTLS := true
			if t, ok := backendConfig["use_tls"].(bool); ok {
				useTLS = t
			}

			emailBackend, err := backends.NewEmailBackend(&backends.EmailConfig{
				SMTPHost:    smtpHost,
				SMTPPort:    smtpPort,
				Username:    username,
				PasswordEnv: password,
				FromAddress: fromAddress,
				UseTLS:      useTLS,
			})
			if err != nil {
				return nil, fmt.Errorf("initializing email backend: %w", err)
			}
			result = append(result, emailBackend)

		case "webhook":
			url := envFromConfig(backendConfig, "url_env_var")
			authToken := envFromConfig(backendConfig, "auth_token_env_var")
			timeoutSec := 30
			if t, ok := backendConfig["timeout_seconds"].(float64); ok {
				timeoutSec = int(t)
			}

			webhookBackend, err := backends.NewWebhookBackend(&backends.WebhookConfig{
				URL:            url,
				AuthTokenEnv:   authToken,
				TimeoutSeconds: timeoutSec,
			})
			if err != nil {
				return nil, fmt.Errorf("initializing webhook backend: %w", err)
			}
			result = append(result, webhookBackend)

		default:
			return nil, fmt.Errorf("unknown backend type %q", backendType)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no backends successfully initialized")
	}

	return result, nil
}

// readEnabledTypes reads the notification_types config and returns a map
// of enabled trigger types.
func readEnabledTypes(config *runner.RunnerConfig) map[string]bool {
	defaults := map[string]bool{
		"change_set_pending_approval": true,
		"change_set_approved":         true,
		"change_set_rejected":         true,
		"emergency_change_filed":      true,
		"compliance_finding_created":  true,
		"escalation_overdue":          true,
	}

	typesConfig, ok := runner.GetSpecDataMap(config, "notification_types")
	if !ok {
		return defaults
	}

	result := make(map[string]bool)
	for key, val := range typesConfig {
		if b, ok := val.(bool); ok {
			result[key] = b
		}
	}

	// Merge defaults for any missing keys.
	for key, val := range defaults {
		if _, exists := result[key]; !exists {
			result[key] = val
		}
	}

	return result
}

// envFromConfig reads a config key that names an environment variable,
// then reads the environment variable. Returns the env var value or empty string.
func envFromConfig(config map[string]interface{}, key string) string {
	envVar, ok := config[key].(string)
	if !ok || envVar == "" {
		return ""
	}
	return os.Getenv(envVar)
}
