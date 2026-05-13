package oncall

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	runner "github.com/ghowland/opsdb/tools/opsdb_runner_lib"
)

// ImportOpsgenieSchedules reads on-call schedules from the Opsgenie API
// and maps them to on_call_schedule observations.
func ImportOpsgenieSchedules(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOpsgenieClient(config)
	offset := 0

	for {
		schedules, total, err := client.ListSchedules(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("opsgenie list schedules offset=%d: %w", offset, err)
		}

		for _, sched := range schedules {
			rotationNames := make([]string, 0, len(sched.Rotations))
			for _, rot := range sched.Rotations {
				rotationNames = append(rotationNames, rot.Name)
			}

			obs := Observation{
				EntityType: "on_call_schedule",
				EntityID:   sched.ID,
				StateKey:   "opsgenie_schedule",
				Value:      sched.Name,
				DataJSON: map[string]interface{}{
					"name":            sched.Name,
					"timezone":        sched.Timezone,
					"enabled":         sched.Enabled,
					"rotation_count":  len(sched.Rotations),
					"rotation_names":  rotationNames,
					"description":     sched.Description,
					"opsgenie_id":     sched.ID,
					"owner_team_id":   sched.OwnerTeamID,
					"owner_team_name": sched.OwnerTeamName,
				},
			}
			results = append(results, obs)
		}

		offset += len(schedules)
		if offset >= total {
			break
		}
	}

	return results, nil
}

// ImportOpsgenieAssignments reads current on-call assignments for all schedules
// from the Opsgenie API and maps them to on_call_assignment observations.
func ImportOpsgenieAssignments(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOpsgenieClient(config)
	offset := 0

	for {
		schedules, total, err := client.ListSchedules(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("opsgenie list schedules for assignments offset=%d: %w", offset, err)
		}

		for _, sched := range schedules {
			participants, err := client.GetOnCallParticipants(sched.ID)
			if err != nil {
				results = append(results, Observation{
					EntityType: "on_call_assignment",
					EntityID:   fmt.Sprintf("%s_error", sched.ID),
					StateKey:   "opsgenie_assignment_error",
					Value:      fmt.Sprintf("failed to read participants: %v", err),
					DataJSON: map[string]interface{}{
						"schedule_id": sched.ID,
						"error":       err.Error(),
					},
				})
				continue
			}

			for i, participant := range participants {
				assignmentID := fmt.Sprintf("%s_%d", sched.ID, i)
				obs := Observation{
					EntityType: "on_call_assignment",
					EntityID:   assignmentID,
					StateKey:   "opsgenie_assignment",
					Value:      participant.Name,
					DataJSON: map[string]interface{}{
						"schedule_id":      sched.ID,
						"schedule_name":    sched.Name,
						"participant_name": participant.Name,
						"participant_id":   participant.ID,
						"participant_type": participant.Type,
						"rotation_id":      participant.RotationID,
						"rotation_name":    participant.RotationName,
						"start_time":       participant.StartTime.Format(time.RFC3339),
						"end_time":         participant.EndTime.Format(time.RFC3339),
						"opsgenie_id":      sched.ID,
					},
				}
				results = append(results, obs)
			}
		}

		offset += len(schedules)
		if offset >= total {
			break
		}
	}

	return results, nil
}

// ImportOpsgenieEscalations reads escalation policies from the Opsgenie API
// and maps them to escalation_path observations.
func ImportOpsgenieEscalations(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOpsgenieClient(config)
	offset := 0

	for {
		policies, total, err := client.ListEscalations(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("opsgenie list escalations offset=%d: %w", offset, err)
		}

		for _, policy := range policies {
			obs := Observation{
				EntityType: "escalation_path",
				EntityID:   policy.ID,
				StateKey:   "opsgenie_escalation",
				Value:      policy.Name,
				DataJSON: map[string]interface{}{
					"name":            policy.Name,
					"description":     policy.Description,
					"rule_count":      len(policy.Rules),
					"owner_team_id":   policy.OwnerTeamID,
					"owner_team_name": policy.OwnerTeamName,
					"repeat_count":    policy.RepeatCount,
					"opsgenie_id":     policy.ID,
				},
			}
			results = append(results, obs)
		}

		offset += len(policies)
		if offset >= total {
			break
		}
	}

	return results, nil
}

// ImportOpsgenieEscalationSteps reads escalation rules (steps) from each
// escalation policy and maps them to escalation_step observations.
func ImportOpsgenieEscalationSteps(config *ImportConfig) ([]Observation, error) {
	var results []Observation

	client := newOpsgenieClient(config)
	offset := 0

	for {
		policies, total, err := client.ListEscalations(offset, config.BatchSize)
		if err != nil {
			return results, fmt.Errorf("opsgenie list escalations for steps offset=%d: %w", offset, err)
		}

		for _, policy := range policies {
			for stepOrder, rule := range policy.Rules {
				recipientNames := make([]string, 0, len(rule.Recipients))
				for _, r := range rule.Recipients {
					recipientNames = append(recipientNames, r.Name)
				}

				stepID := fmt.Sprintf("%s_step_%d", policy.ID, stepOrder)
				obs := Observation{
					EntityType: "escalation_step",
					EntityID:   stepID,
					StateKey:   "opsgenie_escalation_step",
					Value:      fmt.Sprintf("step %d of %s", stepOrder+1, policy.Name),
					DataJSON: map[string]interface{}{
						"escalation_path_id":   policy.ID,
						"escalation_path_name": policy.Name,
						"step_order":           stepOrder + 1,
						"delay_minutes":        rule.DelayMinutes,
						"notify_type":          rule.NotifyType,
						"recipient_count":      len(rule.Recipients),
						"recipient_names":      recipientNames,
						"opsgenie_rule_id":     rule.ID,
					},
				}
				results = append(results, obs)
			}
		}

		offset += len(policies)
		if offset >= total {
			break
		}
	}

	return results, nil
}

// opsgenieClient wraps Opsgenie API access with authentication and retry.
type opsgenieClient struct {
	apiToken    string
	baseURL     string
	maxRetries  int
	retryConfig runner.RetryConfig
}

func newOpsgenieClient(config *ImportConfig) *opsgenieClient {
	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.opsgenie.com/v2"
	}
	return &opsgenieClient{
		apiToken:   config.APIToken,
		baseURL:    baseURL,
		maxRetries: config.MaxRetries,
		retryConfig: runner.RetryConfig{
			MaxAttempts:  config.MaxRetries,
			BaseDelay:    time.Second,
			Multiplier:   2.0,
			JitterFrac:   0.25,
			MaxTotalTime: 30 * time.Second,
		},
	}
}

// OpsgenieSchedule represents a schedule from the Opsgenie API.
type OpsgenieSchedule struct {
	ID            string
	Name          string
	Description   string
	Timezone      string
	Enabled       bool
	OwnerTeamID   string
	OwnerTeamName string
	Rotations     []OpsgenieRotation
}

// OpsgenieRotation represents a rotation within a schedule.
type OpsgenieRotation struct {
	ID   string
	Name string
	Type string
}

// OpsgenieParticipant represents a current on-call participant.
type OpsgenieParticipant struct {
	ID           string
	Name         string
	Type         string
	RotationID   string
	RotationName string
	StartTime    time.Time
	EndTime      time.Time
}

// OpsgenieEscalation represents an escalation policy from the Opsgenie API.
type OpsgenieEscalation struct {
	ID            string
	Name          string
	Description   string
	OwnerTeamID   string
	OwnerTeamName string
	RepeatCount   int
	Rules         []OpsgenieEscalationRule
}

// OpsgenieEscalationRule represents one step in an escalation policy.
type OpsgenieEscalationRule struct {
	ID           string
	DelayMinutes int
	NotifyType   string
	Recipients   []OpsgenieRecipient
}

// OpsgenieRecipient represents a notification target within an escalation rule.
type OpsgenieRecipient struct {
	ID   string
	Name string
	Type string
}

// ListSchedules retrieves a paginated list of schedules from Opsgenie.
func (c *opsgenieClient) ListSchedules(offset int, limit int) ([]OpsgenieSchedule, int, error) {
	var schedules []OpsgenieSchedule
	var total int

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/schedules?offset=%d&limit=%d", c.baseURL, offset, limit)
		resp, err := opsgenieGet(url, c.apiToken)
		if err != nil {
			return fmt.Errorf("opsgenie GET schedules: %w", err)
		}

		total = resp.TotalCount
		for _, item := range resp.Data {
			sched := OpsgenieSchedule{
				ID:            getString(item, "id"),
				Name:          getString(item, "name"),
				Description:   getString(item, "description"),
				Timezone:      getString(item, "timezone"),
				Enabled:       getBool(item, "enabled"),
				OwnerTeamID:   getNestedString(item, "ownerTeam", "id"),
				OwnerTeamName: getNestedString(item, "ownerTeam", "name"),
			}

			if rotations, ok := item["rotations"].([]interface{}); ok {
				for _, r := range rotations {
					if rm, ok := r.(map[string]interface{}); ok {
						sched.Rotations = append(sched.Rotations, OpsgenieRotation{
							ID:   getString(rm, "id"),
							Name: getString(rm, "name"),
							Type: getString(rm, "type"),
						})
					}
				}
			}

			schedules = append(schedules, sched)
		}
		return nil
	})

	return schedules, total, err
}

// GetOnCallParticipants retrieves current on-call participants for a schedule.
func (c *opsgenieClient) GetOnCallParticipants(scheduleID string) ([]OpsgenieParticipant, error) {
	var participants []OpsgenieParticipant

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/schedules/%s/on-calls", c.baseURL, scheduleID)
		resp, err := opsgenieGet(url, c.apiToken)
		if err != nil {
			return fmt.Errorf("opsgenie GET on-calls for schedule %s: %w", scheduleID, err)
		}

		if data, ok := resp.Data[0]["onCallParticipants"].([]interface{}); ok {
			for _, p := range data {
				if pm, ok := p.(map[string]interface{}); ok {
					participant := OpsgenieParticipant{
						ID:           getString(pm, "id"),
						Name:         getString(pm, "name"),
						Type:         getString(pm, "type"),
						RotationID:   getNestedString(pm, "rotation", "id"),
						RotationName: getNestedString(pm, "rotation", "name"),
						StartTime:    getTime(pm, "onCallStart"),
						EndTime:      getTime(pm, "onCallEnd"),
					}
					participants = append(participants, participant)
				}
			}
		}
		return nil
	})

	return participants, err
}

// ListEscalations retrieves a paginated list of escalation policies from Opsgenie.
func (c *opsgenieClient) ListEscalations(offset int, limit int) ([]OpsgenieEscalation, int, error) {
	var escalations []OpsgenieEscalation
	var total int

	err := runner.WithRetry(c.retryConfig, func() error {
		url := fmt.Sprintf("%s/escalations?offset=%d&limit=%d", c.baseURL, offset, limit)
		resp, err := opsgenieGet(url, c.apiToken)
		if err != nil {
			return fmt.Errorf("opsgenie GET escalations: %w", err)
		}

		total = resp.TotalCount
		for _, item := range resp.Data {
			esc := OpsgenieEscalation{
				ID:            getString(item, "id"),
				Name:          getString(item, "name"),
				Description:   getString(item, "description"),
				OwnerTeamID:   getNestedString(item, "ownerTeam", "id"),
				OwnerTeamName: getNestedString(item, "ownerTeam", "name"),
				RepeatCount:   getInt(item, "repeat", "count"),
			}

			if rules, ok := item["rules"].([]interface{}); ok {
				for _, r := range rules {
					if rm, ok := r.(map[string]interface{}); ok {
						rule := OpsgenieEscalationRule{
							ID:           getString(rm, "id"),
							DelayMinutes: getIntDirect(rm, "delay", "timeAmount"),
							NotifyType:   getString(rm, "notifyType"),
						}

						if recipients, ok := rm["recipient"].(map[string]interface{}); ok {
							rule.Recipients = append(rule.Recipients, OpsgenieRecipient{
								ID:   getString(recipients, "id"),
								Name: getString(recipients, "name"),
								Type: getString(recipients, "type"),
							})
						}
						if recipients, ok := rm["recipients"].([]interface{}); ok {
							for _, rec := range recipients {
								if recm, ok := rec.(map[string]interface{}); ok {
									rule.Recipients = append(rule.Recipients, OpsgenieRecipient{
										ID:   getString(recm, "id"),
										Name: getString(recm, "name"),
										Type: getString(recm, "type"),
									})
								}
							}
						}

						esc.Rules = append(esc.Rules, rule)
					}
				}
			}

			escalations = append(escalations, esc)
		}
		return nil
	})

	return escalations, total, err
}

// opsgenieResponse holds a parsed Opsgenie API response.
type opsgenieResponse struct {
	Data       []map[string]interface{}
	TotalCount int
}

// opsgenieGet performs an authenticated GET request against the Opsgenie API.
func opsgenieGet(url string, apiToken string) (*opsgenieResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "GenieKey "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("opsgenie API returned status %d for %s: %s", resp.StatusCode, url, string(body))
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	result := &opsgenieResponse{}

	if data, ok := body["data"].([]interface{}); ok {
		for _, item := range data {
			if m, ok := item.(map[string]interface{}); ok {
				result.Data = append(result.Data, m)
			}
		}
	} else if data, ok := body["data"].(map[string]interface{}); ok {
		result.Data = append(result.Data, data)
	}

	if paging, ok := body["paging"].(map[string]interface{}); ok {
		if total, ok := paging["total"].(float64); ok {
			result.TotalCount = int(total)
		}
	}
	if result.TotalCount == 0 {
		result.TotalCount = len(result.Data)
	}

	return result, nil
}

// getString safely extracts a string from a map.
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getBool safely extracts a bool from a map.
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// getNestedString safely extracts a string from a nested map.
func getNestedString(m map[string]interface{}, outerKey string, innerKey string) string {
	if outer, ok := m[outerKey].(map[string]interface{}); ok {
		return getString(outer, innerKey)
	}
	return ""
}

// getInt safely extracts an int from a nested map path.
func getInt(m map[string]interface{}, keys ...string) int {
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			if v, ok := current[key].(float64); ok {
				return int(v)
			}
			return 0
		}
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return 0
		}
	}
	return 0
}

// getIntDirect safely extracts an int from a nested map with two keys.
func getIntDirect(m map[string]interface{}, outerKey string, innerKey string) int {
	if outer, ok := m[outerKey].(map[string]interface{}); ok {
		if v, ok := outer[innerKey].(float64); ok {
			return int(v)
		}
	}
	return 0
}

// getTime safely extracts a time.Time from a map, parsing RFC3339 format.
func getTime(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// httpNewRequest wraps http.NewRequest for testability.
var httpNewRequest = httpNewRequestDefault

func httpNewRequestDefault(method string, url string, body interface{}) (*httpRequest, error) {
	// implemented via net/http in production
	return nil, fmt.Errorf("http client not initialized")
}

// httpDo wraps http.Client.Do for testability.
var httpDo = httpDoDefault

func httpDoDefault(req *httpRequest) (*httpResponse, error) {
	return nil, fmt.Errorf("http client not initialized")
}

// jsonDecode wraps json.NewDecoder for testability.
var jsonDecode = jsonDecodeDefault

func jsonDecodeDefault(r interface{}, v interface{}) error {
	return fmt.Errorf("json decoder not initialized")
}

// httpRequest is a type alias for http.Request used in the function variable signatures.
type httpRequest = interface{}

// httpResponse wraps the fields we need from http.Response.
type httpResponse struct {
	StatusCode int
	Body       interface{ Close() error }
}
