//# tools/opsdb-api/operations/write_observation.go

package operations

import (
	"net/http"
	"time"

	"github.com/ghowland/opsdb/tools/opsdb-api/gate"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// WriteObservationParams holds write_observation parameters.
type WriteObservationParams struct {
	TargetTable  string                 `json:"target_table"`
	Key          string                 `json:"key"`
	Value        interface{}            `json:"value"`
	DataJSON     map[string]interface{} `json:"data_json"`
	RunnerJobID  int                    `json:"runner_job_id"`
	AuthorityID  int                    `json:"authority_id"`
	Hostname     string                 `json:"hostname"`
	EntityType   string                 `json:"entity_type"`
	EntityID     int                    `json:"entity_id"`
	ObservedTime *string                `json:"observed_time"`
}

// WriteResult holds the result of a write operation.
type WriteResult struct {
	RowID    int  `json:"row_id"`
	Upserted bool `json:"upserted"`
}

// ---------------------------------------------------------------------------
// HTTP handler
// ---------------------------------------------------------------------------

// WriteObservation handles POST /api/v1/observation/write
// Parses the request body, packs all parameters into a GateRequest, and
// lets the gate pipeline handle validation, report key enforcement, audit,
// and the actual database write in step_execute.
func (h *Handlers) WriteObservation(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var params WriteObservationParams
	if err := parseJSONBody(r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false, "error": err.Error(),
		})
		return
	}

	// build the params map that step_execute consumes
	gateParams := map[string]interface{}{
		"target_table":  params.TargetTable,
		"key":           params.Key,
		"value":         params.Value,
		"runner_job_id": params.RunnerJobID,
		"authority_id":  params.AuthorityID,
		"hostname":      params.Hostname,
		"entity_type":   params.EntityType,
		"entity_id":     params.EntityID,
	}

	if params.DataJSON != nil {
		gateParams["data_json"] = params.DataJSON
	}

	if params.ObservedTime != nil {
		gateParams["observed_time"] = *params.ObservedTime
	}

	resp := h.gate.ProcessRequest(&gate.GateRequest{
		Operation:      "write_observation",
		OperationClass: "write-direct",
		TargetEntity:   params.TargetTable,
		Params:         gateParams,
		RawCredentials: parseCredentials(r),
		ClientIP:       clientIP(r),
		UserAgent:      r.UserAgent(),
		RequestID:      newRequestID(),
		ReceivedAt:     time.Now().UTC(),
	})

	writeGateResponse(w, resp)
}
