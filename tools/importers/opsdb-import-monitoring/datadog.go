// === importers/opsdb-import-monitoring/datadog.go ===
package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb-runner-lib"
)

// ImportDatadogMonitors reads monitors from the Datadog API and maps them
// to monitor observations.
func ImportDatadogMonitors(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newDatadogClient(config)
	page := 0

	for {
		monitors, total, err := client.ListMonitors(page, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("datadog list monitors page=%d: %w", page, err)
		}

		for _, mon := range monitors {
			tagList := strings.Join(mon.Tags, ",")

			obs := Observation{
				EntityType: "monitor",
				EntityID:   fmt.Sprintf("dd_%d", mon.ID),
				StateKey:   "datadog_monitor",
				Value:      mon.Name,
				DataJSON: map[string]interface{}{
					"monitor_type":       mapDatadogMonitorType(mon.Type),
					"name":               mon.Name,
					"query":              mon.Query,
					"message":            mon.Message,
					"tags":               mon.Tags,
					"tags_csv":           tagList,
					"datadog_type":       mon.Type,
					"datadog_id":         mon.ID,
					"created":            mon.Created,
					"modified":           mon.Modified,
					"creator_name":       mon.CreatorName,
					"creator_email":      mon.CreatorEmail,
					"overall_state":      mon.OverallState,
					"priority":           mon.Priority,
					"restricted_roles":   mon.RestrictedRoles,
					"multi":              mon.Multi,
				},
			}
			results = append(results, obs)
		}

		page++
		if page*config.BatchSize >= total {
			break
		}
	}

	return results, nil
}

// ImportDatadogAlerts reads monitor definitions and extracts alert thresholds,
// mapping them to alert observations.
func ImportDatadogAlerts(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newDatadogClient(config)
	page := 0

	for {
		monitors, total, err := client.ListMonitors(page, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("datadog list monitors for alerts page=%d: %w", page, err)
		}

		for _, mon := range monitors {
			severity := datadogSeverityFromPriority(mon.Priority)

			obs := Observation{
				EntityType: "alert",
				EntityID:   fmt.Sprintf("dd_%d", mon.ID),
				StateKey:   "datadog_alert_definition",
				Value:      mon.Name,
				DataJSON: map[string]interface{}{
					"name":                  mon.Name,
					"severity":              severity,
					"monitor_id":            fmt.Sprintf("dd_%d", mon.ID),
					"query":                 mon.Query,
					"message":               mon.Message,
					"tags":                  mon.Tags,
					"datadog_type":          mon.Type,
					"datadog_id":            mon.ID,
					"threshold_critical":    mon.ThresholdCritical,
					"threshold_warning":     mon.ThresholdWarning,
					"threshold_ok":          mon.ThresholdOK,
					"notify_no_data":        mon.NotifyNoData,
					"no_data_timeframe":     mon.NoDataTimeframe,
					"renotify_interval":     mon.RenotifyInterval,
					"escalation_message":    mon.EscalationMessage,
					"evaluation_delay":      mon.EvaluationDelay,
				},
			}
			results = append(results, obs)
		}

		page++
		if page*config.BatchSize >= total {
			break
		}
	}

	return results, nil
}

// ImportDatadogMetrics reads currently triggered monitors from the Datadog API
// and maps them to alert_fire observations.
func ImportDatadogMetrics(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newDatadogClient(config)

	triggeredMonitors, err := client.ListTriggeredMonitors()
	if err != nil {
		return results, fmt.Errorf("datadog triggered monitors: %w", err)
	}

	for _, mon := range triggeredMonitors {
		for groupName, groupState := range mon.GroupStates {
			fireID := fmt.Sprintf("dd_%d_%s", mon.ID, sanitizeID(groupName))

			obs := Observation{
				EntityType: "alert_fire",
				EntityID:   fireID,
				StateKey:   "datadog_alert_fire",
				Value:      mon.Name,
				DataJSON: map[string]interface{}{
					"alert_name":       mon.Name,
					"state":            groupState.Status,
					"group_name":       groupName,
					"last_triggered":   groupState.LastTriggered,
					"last_resolved":    groupState.LastResolved,
					"last_notified":    groupState.LastNotified,
					"datadog_id":       mon.ID,
					"datadog_type":     mon.Type,
					"query":            mon.Query,
					"tags":             mon.Tags,
					"overall_state":    mon.OverallState,
				},
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

// datadogClient wraps Datadog API access with authentication and retry.
type datadogClient struct {
	apiKey      string
	appKey      string
	baseURL     string
	retryConfig runner.RetryConfig
}

func newDatadogClient(config *ImportConfig) *datadogClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.datadoghq.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &datadogClient{
		apiKey:  config.APIToken,
		appKey:  config.AppKey,
		baseURL: baseURL,
		retryConfig: runner.RetryConfig{
			MaxAttempts:  config.MaxRetries,
			BaseDelay:    time.Second,
			Multiplier:   2.0,
			JitterFrac:   0.25,
			MaxTotalTime: 30 * time.Second,
		},
	}
}

// ddMonitor represents a monitor from the Datadog API.
type ddMonitor struct {
	ID                int
	Name              string
	Type              string
	Query             string
	Message           string
	Tags              []string
	Created           string
	Modified          string
	CreatorName       string
	CreatorEmail      string
	OverallState      string
	Priority          int
	Multi             bool
	RestrictedRoles   []string
	ThresholdCritical *float64
	ThresholdWarning  *float64
	ThresholdOK       *float64
	NotifyNoData      bool
	NoDataTimeframe   int
	RenotifyInterval  int
	EscalationMessage string
	EvaluationDelay   int
}

// ddTriggeredMonitor represents a triggered monitor with per-group state.
type ddTriggeredMonitor struct {
	ID           int
	Name         string
	Type         string
	Query        string
	Tags         []string
	OverallState string
	GroupStates  map[string]ddGroupState
}

// ddGroupState holds the state for one group within a triggered monitor.
type ddGroupState struct {
	Status        string
	LastTriggered string
	LastResolved  string
	LastNotified  string
}

// ListMonitors retrieves a paginated list of monitors from Datadog.
func (c *datadogClient) ListMonitors(page int, pageSize int) ([]ddMonitor, int, error) {
	var monitors []ddMonitor
	var total int

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/api/v1/monitor?page=%d&page_size=%d", c.baseURL, page, pageSize)
		body, headers, err := datadogGet(url, c.apiKey, c.appKey)
		if err != nil {
			return fmt.Errorf("datadog GET monitors: %w", err)
		}

		items, ok := body.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from monitors endpoint")
		}

		// Datadog returns total count in a response header
		if countStr := headers.Get("X-Total-Count"); countStr != "" {
			fmt.Sscanf(countStr, "%d", &total)
		}
		if total == 0 {
			total = len(items)
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			mon := ddMonitor{
				ID:           ddGetInt(m, "id"),
				Name:         ddGetString(m, "name"),
				Type:         ddGetString(m, "type"),
				Query:        ddGetString(m, "query"),
				Message:      ddGetString(m, "message"),
				Created:      ddGetString(m, "created"),
				Modified:     ddGetString(m, "modified"),
				OverallState: ddGetString(m, "overall_state"),
				Priority:     ddGetInt(m, "priority"),
				Multi:        ddGetBool(m, "multi"),
			}

			if tags, ok := m["tags"].([]interface{}); ok {
				for _, t := range tags {
					if s, ok := t.(string); ok {
						mon.Tags = append(mon.Tags, s)
					}
				}
			}

			if roles, ok := m["restricted_roles"].([]interface{}); ok {
				for _, r := range roles {
					if s, ok := r.(string); ok {
						mon.RestrictedRoles = append(mon.RestrictedRoles, s)
					}
				}
			}

			if creator, ok := m["creator"].(map[string]interface{}); ok {
				mon.CreatorName = ddGetString(creator, "name")
				mon.CreatorEmail = ddGetString(creator, "email")
			}

			if options, ok := m["options"].(map[string]interface{}); ok {
				mon.NotifyNoData = ddGetBool(options, "notify_no_data")
				mon.NoDataTimeframe = ddGetInt(options, "no_data_timeframe")
				mon.RenotifyInterval = ddGetInt(options, "renotify_interval")
				mon.EscalationMessage = ddGetString(options, "escalation_message")
				mon.EvaluationDelay = ddGetInt(options, "evaluation_delay")

				if thresholds, ok := options["thresholds"].(map[string]interface{}); ok {
					if v, ok := thresholds["critical"].(float64); ok {
						mon.ThresholdCritical = &v
					}
					if v, ok := thresholds["warning"].(float64); ok {
						mon.ThresholdWarning = &v
					}
					if v, ok := thresholds["ok"].(float64); ok {
						mon.ThresholdOK = &v
					}
				}
			}

			monitors = append(monitors, mon)
		}
		return nil
	})

	return monitors, total, err
}

// ListTriggeredMonitors retrieves monitors currently in a triggered state.
func (c *datadogClient) ListTriggeredMonitors() ([]ddTriggeredMonitor, error) {
	var results []ddTriggeredMonitor

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/api/v1/monitor?group_states=alert,warn,no+data", c.baseURL)
		body, _, err := datadogGet(url, c.apiKey, c.appKey)
		if err != nil {
			return fmt.Errorf("datadog GET triggered monitors: %w", err)
		}

		items, ok := body.([]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from triggered monitors")
		}

		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			overallState := ddGetString(m, "overall_state")
			if overallState != "Alert" && overallState != "Warn" && overallState != "No Data" {
				continue
			}

			triggered := ddTriggeredMonitor{
				ID:           ddGetInt(m, "id"),
				Name:         ddGetString(m, "name"),
				Type:         ddGetString(m, "type"),
				Query:        ddGetString(m, "query"),
				OverallState: overallState,
				GroupStates:  make(map[string]ddGroupState),
			}

			if tags, ok := m["tags"].([]interface{}); ok {
				for _, t := range tags {
					if s, ok := t.(string); ok {
						triggered.Tags = append(triggered.Tags, s)
					}
				}
			}

			if state, ok := m["state"].(map[string]interface{}); ok {
				if groups, ok := state["groups"].(map[string]interface{}); ok {
					for groupName, groupData := range groups {
						gm, ok := groupData.(map[string]interface{})
						if !ok {
							continue
						}
						triggered.GroupStates[groupName] = ddGroupState{
							Status:        ddGetString(gm, "status"),
							LastTriggered: ddGetString(gm, "last_triggered_ts"),
							LastResolved:  ddGetString(gm, "last_resolved_ts"),
							LastNotified:  ddGetString(gm, "last_notified_ts"),
						}
					}
				}
			}

			// if no group states parsed but monitor is triggered, add a default entry
			if len(triggered.GroupStates) == 0 {
				triggered.GroupStates["*"] = ddGroupState{
					Status: overallState,
				}
			}

			results = append(results, triggered)
		}
		return nil
	})

	return results, err
}

// datadogGet performs an authenticated GET request against the Datadog API.
func datadogGet(url string, apiKey string, appKey string) (interface{}, http.Header, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("DD-API-KEY", apiKey)
	req.Header.Set("DD-APPLICATION-KEY", appKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, resp.Header, fmt.Errorf("datadog rate limited (429) for %s", url)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, resp.Header, fmt.Errorf("datadog API returned status %d for %s: %s", resp.StatusCode, url, string(bodyBytes))
	}

	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, resp.Header, fmt.Errorf("decoding response: %w", err)
	}

	return body, resp.Header, nil
}

// mapDatadogMonitorType maps a Datadog monitor type to the OpsDB monitor_type discriminator.
func mapDatadogMonitorType(ddType string) string {
	switch ddType {
	case "metric alert":
		return "cloud_metric"
	case "query alert":
		return "prometheus_query"
	case "service check":
		return "script_remote"
	case "host":
		return "script_remote"
	case "process alert":
		return "script_remote"
	case "synthetics alert":
		return "http_probe"
	case "log alert":
		return "cloud_metric"
	case "rum alert":
		return "cloud_metric"
	case "apm alert":
		return "cloud_metric"
	case "composite":
		return "cloud_metric"
	default:
		return "cloud_metric"
	}
}

// datadogSeverityFromPriority maps Datadog monitor priority (1-5) to a severity string.
func datadogSeverityFromPriority(priority int) string {
	switch priority {
	case 1:
		return "critical"
	case 2:
		return "high"
	case 3:
		return "warning"
	case 4:
		return "low"
	case 5:
		return "info"
	default:
		return "warning"
	}
}

// ddGetString safely extracts a string from a map.
func ddGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// ddGetInt safely extracts an int from a map.
func ddGetInt(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

// ddGetBool safely extracts a bool from a map.
func ddGetBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}