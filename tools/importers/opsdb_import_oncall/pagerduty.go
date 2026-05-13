// === importers/opsdb_import_oncall/pagerduty.go ===
package oncall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// ImportConfig holds on-call importer configuration shared across backends.
type ImportConfig struct {
	Backend              string
	APIToken             string
	BaseURL              string
	BatchSize            int
	MaxRetries           int
	AssignmentWindowDays int
}

// Observation is the observation structure for on-call importers.
type Observation struct {
	EntityType string
	EntityID   string
	StateKey   string
	Value      string
	DataJSON   map[string]interface{}
}

// ImportPagerDutySchedules reads on-call schedules from the PagerDuty API
// and maps them to on_call_schedule observations.
func ImportPagerDutySchedules(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPagerDutyClient(config)
	offset := 0
	more := true

	for more {
		schedules, hasMore, err := client.ListSchedules(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("pagerduty list schedules offset=%d: %w", offset, err)
		}

		for _, sched := range schedules {
			layerNames := make([]string, 0, len(sched.ScheduleLayers))
			for _, layer := range sched.ScheduleLayers {
				layerNames = append(layerNames, layer.Name)
			}

			obs := Observation{
				EntityType: "on_call_schedule",
				EntityID:   sched.ID,
				StateKey:   "pagerduty_schedule",
				Value:      sched.Name,
				DataJSON: map[string]interface{}{
					"name":               sched.Name,
					"description":        sched.Description,
					"timezone":           sched.TimeZone,
					"layer_count":        len(sched.ScheduleLayers),
					"layer_names":        layerNames,
					"pagerduty_id":       sched.ID,
					"pagerduty_html_url": sched.HTMLURL,
				},
			}
			results = append(results, obs)
		}

		offset += len(schedules)
		more = hasMore
	}

	return results, nil
}

// ImportPagerDutyAssignments reads current and upcoming on-call assignments
// from the PagerDuty API and maps them to on_call_assignment observations.
func ImportPagerDutyAssignments(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPagerDutyClient(config)
	offset := 0
	more := true

	windowDays := config.AssignmentWindowDays
	if windowDays <= 0 {
		windowDays = 14
	}
	since := time.Now().UTC()
	until := since.AddDate(0, 0, windowDays)

	// first collect all schedule IDs
	var scheduleIDs []string
	for more {
		schedules, hasMore, err := client.ListSchedules(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("pagerduty list schedules for assignments offset=%d: %w", offset, err)
		}
		for _, sched := range schedules {
			scheduleIDs = append(scheduleIDs, sched.ID)
		}
		offset += len(schedules)
		more = hasMore
	}

	for _, schedID := range scheduleIDs {
		entries, scheduleName, err := client.ListOnCalls(schedID, since, until)
		if err != nil {
			results = append(results, Observation{
				EntityType: "on_call_assignment",
				EntityID:   fmt.Sprintf("%s_error", schedID),
				StateKey:   "pagerduty_assignment_error",
				Value:      fmt.Sprintf("failed to read on-calls: %v", err),
				DataJSON: map[string]interface{}{
					"schedule_id": schedID,
					"error":       err.Error(),
				},
			})
			continue
		}

		for i, entry := range entries {
			assignmentID := fmt.Sprintf("%s_%d", schedID, i)
			obs := Observation{
				EntityType: "on_call_assignment",
				EntityID:   assignmentID,
				StateKey:   "pagerduty_assignment",
				Value:      entry.UserName,
				DataJSON: map[string]interface{}{
					"schedule_id":           schedID,
					"schedule_name":         scheduleName,
					"user_name":             entry.UserName,
					"user_id":               entry.UserID,
					"user_email":            entry.UserEmail,
					"start_time":            entry.Start.Format(time.RFC3339),
					"end_time":              entry.End.Format(time.RFC3339),
					"escalation_level":      entry.EscalationLevel,
					"escalation_policy_id":  entry.EscalationPolicyID,
					"pagerduty_schedule_id": schedID,
				},
			}
			results = append(results, obs)
		}
	}

	return results, nil
}

// ImportPagerDutyEscalations reads escalation policies from the PagerDuty API
// and maps them to escalation_path observations.
func ImportPagerDutyEscalations(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPagerDutyClient(config)
	offset := 0
	more := true

	for more {
		policies, hasMore, err := client.ListEscalationPolicies(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("pagerduty list escalation policies offset=%d: %w", offset, err)
		}

		for _, policy := range policies {
			serviceIDs := make([]string, 0, len(policy.Services))
			for _, svc := range policy.Services {
				serviceIDs = append(serviceIDs, svc.ID)
			}

			obs := Observation{
				EntityType: "escalation_path",
				EntityID:   policy.ID,
				StateKey:   "pagerduty_escalation",
				Value:      policy.Name,
				DataJSON: map[string]interface{}{
					"name":               policy.Name,
					"description":        policy.Description,
					"num_loops":          policy.NumLoops,
					"rule_count":         len(policy.Rules),
					"service_count":      len(policy.Services),
					"service_ids":        serviceIDs,
					"on_call_handoff":    policy.OnCallHandoff,
					"pagerduty_id":       policy.ID,
					"pagerduty_html_url": policy.HTMLURL,
				},
			}
			results = append(results, obs)
		}

		offset += len(policies)
		more = hasMore
	}

	return results, nil
}

// ImportPagerDutyEscalationSteps reads escalation rules from each policy
// and maps them to escalation_step observations.
func ImportPagerDutyEscalationSteps(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newPagerDutyClient(config)
	offset := 0
	more := true

	for more {
		policies, hasMore, err := client.ListEscalationPolicies(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("pagerduty list escalation policies for steps offset=%d: %w", offset, err)
		}

		for _, policy := range policies {
			for stepOrder, rule := range policy.Rules {
				targetNames := make([]string, 0, len(rule.Targets))
				targetTypes := make([]string, 0, len(rule.Targets))
				for _, target := range rule.Targets {
					targetNames = append(targetNames, target.Name)
					targetTypes = append(targetTypes, target.Type)
				}

				stepID := fmt.Sprintf("%s_step_%d", policy.ID, stepOrder)
				obs := Observation{
					EntityType: "escalation_step",
					EntityID:   stepID,
					StateKey:   "pagerduty_escalation_step",
					Value:      fmt.Sprintf("step %d of %s", stepOrder+1, policy.Name),
					DataJSON: map[string]interface{}{
						"escalation_path_id":       policy.ID,
						"escalation_path_name":     policy.Name,
						"step_order":               stepOrder + 1,
						"escalation_delay_minutes": rule.EscalationDelayMinutes,
						"target_count":             len(rule.Targets),
						"target_names":             targetNames,
						"target_types":             targetTypes,
						"pagerduty_rule_id":        rule.ID,
					},
				}
				results = append(results, obs)
			}
		}

		offset += len(policies)
		more = hasMore
	}

	return results, nil
}

// pagerDutyClient wraps PagerDuty API access with authentication and retry.
type pagerDutyClient struct {
	apiToken    string
	baseURL     string
	retryConfig runner.RetryConfig
}

func newPagerDutyClient(config *ImportConfig) *pagerDutyClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.pagerduty.com"
	}
	return &pagerDutyClient{
		apiToken: config.APIToken,
		baseURL:  baseURL,
		retryConfig: runner.RetryConfig{
			MaxAttempts:  config.MaxRetries,
			BaseDelay:    time.Second,
			Multiplier:   2.0,
			JitterFrac:   0.25,
			MaxTotalTime: 30 * time.Second,
		},
	}
}

// PDSchedule represents a schedule from the PagerDuty API.
type PDSchedule struct {
	ID             string
	Name           string
	Description    string
	TimeZone       string
	HTMLURL        string
	ScheduleLayers []PDScheduleLayer
}

// PDScheduleLayer represents a layer within a schedule.
type PDScheduleLayer struct {
	ID   string
	Name string
}

// PDOnCallEntry represents one on-call assignment period.
type PDOnCallEntry struct {
	UserName           string
	UserID             string
	UserEmail          string
	Start              time.Time
	End                time.Time
	EscalationLevel    int
	EscalationPolicyID string
}

// PDEscalationPolicy represents an escalation policy from the PagerDuty API.
type PDEscalationPolicy struct {
	ID            string
	Name          string
	Description   string
	NumLoops      int
	OnCallHandoff string
	HTMLURL       string
	Rules         []PDEscalationRule
	Services      []PDServiceRef
}

// PDEscalationRule represents one rule in an escalation policy.
type PDEscalationRule struct {
	ID                     string
	EscalationDelayMinutes int
	Targets                []PDTarget
}

// PDTarget represents a target within an escalation rule.
type PDTarget struct {
	ID   string
	Name string
	Type string
}

// PDServiceRef represents a service reference within an escalation policy.
type PDServiceRef struct {
	ID   string
	Name string
}

// ListSchedules retrieves a paginated list of schedules from PagerDuty.
func (c *pagerDutyClient) ListSchedules(offset int, limit int) ([]PDSchedule, bool, error) {
	var schedules []PDSchedule
	var hasMore bool

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/schedules?offset=%d&limit=%d", c.baseURL, offset, limit)
		body, err := pagerdutyGet(url, c.apiToken)
		if err != nil {
			return fmt.Errorf("pagerduty GET schedules: %w", err)
		}

		hasMore = paginationHasMore(body)

		items, _ := body["schedules"].([]interface{})
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			sched := PDSchedule{
				ID:          getString(m, "id"),
				Name:        getString(m, "name"),
				Description: getString(m, "description"),
				TimeZone:    getString(m, "time_zone"),
				HTMLURL:     getString(m, "html_url"),
			}

			if layers, ok := m["schedule_layers"].([]interface{}); ok {
				for _, l := range layers {
					if lm, ok := l.(map[string]interface{}); ok {
						sched.ScheduleLayers = append(sched.ScheduleLayers, PDScheduleLayer{
							ID:   getString(lm, "id"),
							Name: getString(lm, "name"),
						})
					}
				}
			}

			schedules = append(schedules, sched)
		}
		return nil
	})

	return schedules, hasMore, err
}

// ListOnCalls retrieves on-call entries for a schedule within a time window.
func (c *pagerDutyClient) ListOnCalls(scheduleID string, since time.Time, until time.Time) ([]PDOnCallEntry, string, error) {
	var entries []PDOnCallEntry
	var scheduleName string

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/oncalls?schedule_ids[]=%s&since=%s&until=%s",
			c.baseURL, scheduleID,
			since.Format(time.RFC3339), until.Format(time.RFC3339))
		body, err := pagerdutyGet(url, c.apiToken)
		if err != nil {
			return fmt.Errorf("pagerduty GET oncalls for schedule %s: %w", scheduleID, err)
		}

		items, _ := body["oncalls"].([]interface{})
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			entry := PDOnCallEntry{
				Start:           getTimeField(m, "start"),
				End:             getTimeField(m, "end"),
				EscalationLevel: getIntField(m, "escalation_level"),
			}

			if user, ok := m["user"].(map[string]interface{}); ok {
				entry.UserName = getString(user, "name")
				entry.UserID = getString(user, "id")
				entry.UserEmail = getString(user, "email")
			}

			if ep, ok := m["escalation_policy"].(map[string]interface{}); ok {
				entry.EscalationPolicyID = getString(ep, "id")
			}

			if sched, ok := m["schedule"].(map[string]interface{}); ok {
				if scheduleName == "" {
					scheduleName = getString(sched, "summary")
				}
			}

			entries = append(entries, entry)
		}
		return nil
	})

	return entries, scheduleName, err
}

// ListEscalationPolicies retrieves a paginated list of escalation policies from PagerDuty.
func (c *pagerDutyClient) ListEscalationPolicies(offset int, limit int) ([]PDEscalationPolicy, bool, error) {
	var policies []PDEscalationPolicy
	var hasMore bool

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/escalation_policies?offset=%d&limit=%d", c.baseURL, offset, limit)
		body, err := pagerdutyGet(url, c.apiToken)
		if err != nil {
			return fmt.Errorf("pagerduty GET escalation_policies: %w", err)
		}

		hasMore = paginationHasMore(body)

		items, _ := body["escalation_policies"].([]interface{})
		for _, item := range items {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			policy := PDEscalationPolicy{
				ID:            getString(m, "id"),
				Name:          getString(m, "name"),
				Description:   getString(m, "description"),
				NumLoops:      getIntField(m, "num_loops"),
				OnCallHandoff: getString(m, "on_call_handoff_notifications"),
				HTMLURL:       getString(m, "html_url"),
			}

			if rules, ok := m["escalation_rules"].([]interface{}); ok {
				for _, r := range rules {
					rm, ok := r.(map[string]interface{})
					if !ok {
						continue
					}

					rule := PDEscalationRule{
						ID:                     getString(rm, "id"),
						EscalationDelayMinutes: getIntField(rm, "escalation_delay_in_minutes"),
					}

					if targets, ok := rm["targets"].([]interface{}); ok {
						for _, t := range targets {
							if tm, ok := t.(map[string]interface{}); ok {
								rule.Targets = append(rule.Targets, PDTarget{
									ID:   getString(tm, "id"),
									Name: getString(tm, "name"),
									Type: getString(tm, "type"),
								})
							}
						}
					}

					policy.Rules = append(policy.Rules, rule)
				}
			}

			if services, ok := m["services"].([]interface{}); ok {
				for _, s := range services {
					if sm, ok := s.(map[string]interface{}); ok {
						policy.Services = append(policy.Services, PDServiceRef{
							ID:   getString(sm, "id"),
							Name: getString(sm, "name"),
						})
					}
				}
			}

			policies = append(policies, policy)
		}
		return nil
	})

	return policies, hasMore, err
}

// pagerdutyGet performs an authenticated GET request against the PagerDuty API.
func pagerdutyGet(url string, apiToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Token token="+apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.pagerduty+json;version=2")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("pagerduty rate limited (429) for %s", url)
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pagerduty API returned status %d for %s: %s", resp.StatusCode, url, string(bodyBytes))
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return body, nil
}

// paginationHasMore checks PagerDuty's pagination response for more pages.
func paginationHasMore(body map[string]interface{}) bool {
	if v, ok := body["more"].(bool); ok {
		return v
	}
	return false
}

// getTimeField safely extracts a time.Time from a map field, parsing RFC3339.
func getTimeField(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// getIntField safely extracts an int from a map field.
func getIntField(m map[string]interface{}, key string) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}
