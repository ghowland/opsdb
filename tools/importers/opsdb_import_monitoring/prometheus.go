// === importers/opsdb-import-monitoring/prometheus.go ===
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

// ImportConfig holds monitoring importer configuration shared across backends.
type ImportConfig struct {
	Backend        string
	APIToken       string
	BaseURL        string
	BatchSize      int
	MaxRetries     int
	MetricPrefixes []string
	ScrapeInterval int
	AppKey         string
}

// Observation is the observation structure for monitoring importers.
type Observation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportPrometheusConfigs reads the Prometheus server configuration and scrape
// targets, mapping them to prometheus_config and prometheus_scrape_target observations.
func ImportPrometheusConfigs(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPrometheusClient(config)

	// read server config
	statusConfig, err := client.GetStatusConfig()
	if err != nil {
		return results, fmt.Errorf("prometheus status/config: %w", err)
	}

	configObs := Observation{
		EntityType: "prometheus_config",
		EntityID:   client.serverID(),
		StateKey:   "prometheus_server_config",
		Value:      client.baseURL,
		DataJSON: map[string]interface{}{
			"base_url":          client.baseURL,
			"config_yaml_hash":  hashString(statusConfig.YAML),
			"config_yaml_bytes": len(statusConfig.YAML),
		},
	}
	results = append(results, configObs)

	// read scrape targets
	targets, err := client.GetTargets()
	if err != nil {
		return results, fmt.Errorf("prometheus targets: %w", err)
	}

	for _, target := range targets {
		targetID := fmt.Sprintf("%s_%s", client.serverID(), sanitizeID(target.ScrapeURL))
		obs := Observation{
			EntityType: "prometheus_scrape_target",
			EntityID:   targetID,
			StateKey:   "prometheus_scrape_target",
			Value:      target.ScrapeURL,
			DataJSON: map[string]interface{}{
				"scrape_url":         target.ScrapeURL,
				"job_name":           target.JobName,
				"health":             target.Health,
				"scrape_interval":    target.ScrapeInterval,
				"scrape_timeout":     target.ScrapeTimeout,
				"scheme":             target.Scheme,
				"metrics_path":       target.MetricsPath,
				"last_scrape_time":   target.LastScrape.Format(time.RFC3339),
				"last_scrape_duration_seconds": target.LastScrapeDuration,
				"last_error":         target.LastError,
				"labels":             target.Labels,
				"discovered_labels":  target.DiscoveredLabels,
				"prometheus_server":  client.baseURL,
			},
		}
		results = append(results, obs)
	}

	return results, nil
}

// ImportPrometheusAlerts reads alerting rules and currently firing alerts from
// Prometheus, mapping them to monitor, alert, and alert_fire observations.
func ImportPrometheusAlerts(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPrometheusClient(config)

	// read alerting rules
	ruleGroups, err := client.GetRules()
	if err != nil {
		return results, fmt.Errorf("prometheus rules: %w", err)
	}

	for _, group := range ruleGroups {
		for _, rule := range group.Rules {
			if rule.Type != "alerting" {
				continue
			}

			monitorID := fmt.Sprintf("%s_%s_%s", client.serverID(), sanitizeID(group.Name), sanitizeID(rule.Name))

			monitorObs := Observation{
				EntityType: "monitor",
				EntityID:   monitorID,
				StateKey:   "prometheus_alerting_rule",
				Value:      rule.Name,
				DataJSON: map[string]interface{}{
					"monitor_type":       "prometheus_query",
					"name":               rule.Name,
					"query":              rule.Query,
					"duration_seconds":   rule.Duration,
					"group_name":         group.Name,
					"group_file":         group.File,
					"group_interval":     group.Interval,
					"labels":             rule.Labels,
					"annotations":        rule.Annotations,
					"health":             rule.Health,
					"last_evaluation":    rule.LastEvaluation.Format(time.RFC3339),
					"evaluation_time_seconds": rule.EvaluationTime,
					"prometheus_server":  client.baseURL,
				},
			}
			results = append(results, monitorObs)

			severity := "warning"
			if s, ok := rule.Labels["severity"]; ok {
				severity = s
			}

			alertObs := Observation{
				EntityType: "alert",
				EntityID:   monitorID,
				StateKey:   "prometheus_alert_definition",
				Value:      rule.Name,
				DataJSON: map[string]interface{}{
					"name":              rule.Name,
					"severity":          severity,
					"monitor_id":        monitorID,
					"query":             rule.Query,
					"for_duration":      rule.Duration,
					"labels":            rule.Labels,
					"annotations":       rule.Annotations,
					"runbook_url":       rule.Annotations["runbook_url"],
					"summary":           rule.Annotations["summary"],
					"description":       rule.Annotations["description"],
					"prometheus_server": client.baseURL,
				},
			}
			results = append(results, alertObs)
		}
	}

	// read currently firing alerts
	firingAlerts, err := client.GetAlerts()
	if err != nil {
		return results, fmt.Errorf("prometheus alerts: %w", err)
	}

	for _, firing := range firingAlerts {
		fireID := fmt.Sprintf("%s_%s_%s", client.serverID(), sanitizeID(firing.Name), sanitizeID(firing.Fingerprint))

		fireObs := Observation{
			EntityType: "alert_fire",
			EntityID:   fireID,
			StateKey:   "prometheus_alert_fire",
			Value:      firing.Name,
			DataJSON: map[string]interface{}{
				"alert_name":        firing.Name,
				"state":             firing.State,
				"active_at":         firing.ActiveAt.Format(time.RFC3339),
				"value":             firing.Value,
				"labels":            firing.Labels,
				"annotations":       firing.Annotations,
				"fingerprint":       firing.Fingerprint,
				"generator_url":     firing.GeneratorURL,
				"prometheus_server": client.baseURL,
			},
		}
		results = append(results, fireObs)
	}

	return results, nil
}

// ImportPrometheusMetrics reads metric metadata from Prometheus and maps to
// observation_cache_metric observations. Imports metadata (name, type, help)
// not actual metric values.
func ImportPrometheusMetrics(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPrometheusClient(config)

	metadata, err := client.GetMetadata()
	if err != nil {
		return results, fmt.Errorf("prometheus metadata: %w", err)
	}

	for metricName, entries := range metadata {
		if len(config.MetricPrefixes) > 0 && !matchesAnyPrefix(metricName, config.MetricPrefixes) {
			continue
		}

		if len(entries) == 0 {
			continue
		}
		entry := entries[0]

		metricID := fmt.Sprintf("%s_%s", client.serverID(), sanitizeID(metricName))
		obs := Observation{
			EntityType: "observation_cache_metric",
			EntityID:   metricID,
			StateKey:   metricName,
			Value:      entry.Type,
			DataJSON: map[string]interface{}{
				"metric_name":       metricName,
				"metric_type":       entry.Type,
				"help":              entry.Help,
				"unit":              entry.Unit,
				"prometheus_server": client.baseURL,
			},
		}
		results = append(results, obs)
	}

	return results, nil
}

// prometheusClient wraps Prometheus HTTP API access with retry.
type prometheusClient struct {
	baseURL     string
	retryConfig runner.RetryConfig
}

func newPrometheusClient(config *ImportConfig) *prometheusClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:9090"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &prometheusClient{
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

func (c *prometheusClient) serverID() string {
	return sanitizeID(c.baseURL)
}

// promStatusConfig holds the response from /api/v1/status/config.
type promStatusConfig struct {
	YAML string
}

// promTarget holds one scrape target from /api/v1/targets.
type promTarget struct {
	ScrapeURL          string
	JobName            string
	Health             string
	ScrapeInterval     string
	ScrapeTimeout      string
	Scheme             string
	MetricsPath        string
	LastScrape         time.Time
	LastScrapeDuration float64
	LastError          string
	Labels             map[string]string
	DiscoveredLabels   map[string]string
}

// promRuleGroup holds one rule group from /api/v1/rules.
type promRuleGroup struct {
	Name     string
	File     string
	Interval string
	Rules    []promRule
}

// promRule holds one rule within a group.
type promRule struct {
	Name           string
	Query          string
	Duration       float64
	Labels         map[string]string
	Annotations    map[string]string
	Health         string
	Type           string
	LastEvaluation time.Time
	EvaluationTime float64
}

// promFiringAlert holds one currently firing alert from /api/v1/alerts.
type promFiringAlert struct {
	Name         string
	State        string
	ActiveAt     time.Time
	Value        string
	Labels       map[string]string
	Annotations  map[string]string
	Fingerprint  string
	GeneratorURL string
}

// promMetadataEntry holds one entry from /api/v1/metadata.
type promMetadataEntry struct {
	Type string
	Help string
	Unit string
}

// GetStatusConfig queries /api/v1/status/config.
func (c *prometheusClient) GetStatusConfig() (*promStatusConfig, error) {
	var result promStatusConfig

	err := runner.WithRetry(c.retryConfig, func() error {
		body, err := prometheusGet(c.baseURL + "/api/v1/status/config")
		if err != nil {
			return fmt.Errorf("GET status/config: %w", err)
		}

		data, ok := body["data"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from status/config")
		}

		if yaml, ok := data["yaml"].(string); ok {
			result.YAML = yaml
		}
		return nil
	})

	return &result, err
}

// GetTargets queries /api/v1/targets and returns active targets.
func (c *prometheusClient) GetTargets() ([]promTarget, error) {
	var targets []promTarget

	err := runner.WithRetry(c.retryConfig, func() error {
		body, err := prometheusGet(c.baseURL + "/api/v1/targets")
		if err != nil {
			return fmt.Errorf("GET targets: %w", err)
		}

		data, ok := body["data"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from targets")
		}

		activeTargets, ok := data["activeTargets"].([]interface{})
		if !ok {
			return nil
		}

		for _, item := range activeTargets {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			target := promTarget{
				ScrapeURL:          promGetString(m, "scrapeUrl"),
				Health:             promGetString(m, "health"),
				ScrapeInterval:     promGetString(m, "scrapeInterval"),
				ScrapeTimeout:      promGetString(m, "scrapeTimeout"),
				LastError:          promGetString(m, "lastError"),
				LastScrapeDuration: promGetFloat(m, "lastScrapeDuration"),
			}

			if labels, ok := m["labels"].(map[string]interface{}); ok {
				target.Labels = toStringMap(labels)
				target.JobName = target.Labels["job"]
				target.Scheme = target.Labels["__scheme__"]
				target.MetricsPath = target.Labels["__metrics_path__"]
			}

			if dl, ok := m["discoveredLabels"].(map[string]interface{}); ok {
				target.DiscoveredLabels = toStringMap(dl)
			}

			if ls, ok := m["lastScrape"].(string); ok {
				if t, err := time.Parse(time.RFC3339, ls); err == nil {
					target.LastScrape = t
				}
			}

			targets = append(targets, target)
		}
		return nil
	})

	return targets, err
}

// GetRules queries /api/v1/rules and returns rule groups.
func (c *prometheusClient) GetRules() ([]promRuleGroup, error) {
	var groups []promRuleGroup

	err := runner.WithRetry(c.retryConfig, func() error {
		body, err := prometheusGet(c.baseURL + "/api/v1/rules")
		if err != nil {
			return fmt.Errorf("GET rules: %w", err)
		}

		data, ok := body["data"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from rules")
		}

		groupList, ok := data["groups"].([]interface{})
		if !ok {
			return nil
		}

		for _, g := range groupList {
			gm, ok := g.(map[string]interface{})
			if !ok {
				continue
			}

			group := promRuleGroup{
				Name:     promGetString(gm, "name"),
				File:     promGetString(gm, "file"),
				Interval: fmt.Sprintf("%v", gm["interval"]),
			}

			rules, ok := gm["rules"].([]interface{})
			if !ok {
				continue
			}

			for _, r := range rules {
				rm, ok := r.(map[string]interface{})
				if !ok {
					continue
				}

				rule := promRule{
					Name:           promGetString(rm, "name"),
					Query:          promGetString(rm, "query"),
					Duration:       promGetFloat(rm, "duration"),
					Health:         promGetString(rm, "health"),
					Type:           promGetString(rm, "type"),
					EvaluationTime: promGetFloat(rm, "evaluationTime"),
				}

				if labels, ok := rm["labels"].(map[string]interface{}); ok {
					rule.Labels = toStringMap(labels)
				}
				if annotations, ok := rm["annotations"].(map[string]interface{}); ok {
					rule.Annotations = toStringMap(annotations)
				}
				if le, ok := rm["lastEvaluation"].(string); ok {
					if t, err := time.Parse(time.RFC3339, le); err == nil {
						rule.LastEvaluation = t
					}
				}

				group.Rules = append(group.Rules, rule)
			}

			groups = append(groups, group)
		}
		return nil
	})

	return groups, err
}

// GetAlerts queries /api/v1/alerts and returns currently firing alerts.
func (c *prometheusClient) GetAlerts() ([]promFiringAlert, error) {
	var alerts []promFiringAlert

	err := runner.WithRetry(c.retryConfig, func() error {
		body, err := prometheusGet(c.baseURL + "/api/v1/alerts")
		if err != nil {
			return fmt.Errorf("GET alerts: %w", err)
		}

		data, ok := body["data"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from alerts")
		}

		alertList, ok := data["alerts"].([]interface{})
		if !ok {
			return nil
		}

		for _, a := range alertList {
			am, ok := a.(map[string]interface{})
			if !ok {
				continue
			}

			alert := promFiringAlert{
				State:        promGetString(am, "state"),
				Value:        promGetString(am, "value"),
				Fingerprint:  promGetString(am, "fingerprint"),
				GeneratorURL: promGetString(am, "generatorURL"),
			}

			if labels, ok := am["labels"].(map[string]interface{}); ok {
				alert.Labels = toStringMap(labels)
				alert.Name = alert.Labels["alertname"]
			}
			if annotations, ok := am["annotations"].(map[string]interface{}); ok {
				alert.Annotations = toStringMap(annotations)
			}
			if at, ok := am["activeAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339, at); err == nil {
					alert.ActiveAt = t
				}
			}

			alerts = append(alerts, alert)
		}
		return nil
	})

	return alerts, err
}

// GetMetadata queries /api/v1/metadata and returns metric metadata keyed by metric name.
func (c *prometheusClient) GetMetadata() (map[string][]promMetadataEntry, error) {
	result := make(map[string][]promMetadataEntry)

	err := runner.WithRetry(c.retryConfig, func() error {
		body, err := prometheusGet(c.baseURL + "/api/v1/metadata")
		if err != nil {
			return fmt.Errorf("GET metadata: %w", err)
		}

		data, ok := body["data"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected response shape from metadata")
		}

		for metricName, entriesRaw := range data {
			entries, ok := entriesRaw.([]interface{})
			if !ok {
				continue
			}

			for _, e := range entries {
				em, ok := e.(map[string]interface{})
				if !ok {
					continue
				}

				entry := promMetadataEntry{
					Type: promGetString(em, "type"),
					Help: promGetString(em, "help"),
					Unit: promGetString(em, "unit"),
				}
				result[metricName] = append(result[metricName], entry)
			}
		}
		return nil
	})

	return result, err
}

// prometheusGet performs a GET request against the Prometheus HTTP API.
func prometheusGet(url string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus API returned status %d for %s: %s", resp.StatusCode, url, string(bodyBytes))
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if status, ok := body["status"].(string); ok && status != "success" {
		errMsg := ""
		if e, ok := body["error"].(string); ok {
			errMsg = e
		}
		return nil, fmt.Errorf("prometheus API returned status %q: %s", status, errMsg)
	}

	return body, nil
}

// promGetString safely extracts a string from a map.
func promGetString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// promGetFloat safely extracts a float64 from a map.
func promGetFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 0
}

// toStringMap converts a map[string]interface{} to map[string]string.
func toStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// matchesAnyPrefix checks if a string starts with any of the given prefixes.
func matchesAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// sanitizeID replaces characters unsuitable for entity IDs with underscores.
func sanitizeID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// hashString produces a simple hash string for config change detection.
func hashString(s string) string {
	var h int
	for _, c := range s {
		h = h*31 + int(c)
	}
	return fmt.Sprintf("%x", uint(h))
}
