package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

// FlagHandler handles flag-related HTTP requests
type FlagHandler struct {
	evaluator  flags.Evaluator
	outboxRepo ports.OutboxRepository
}

// NewFlagHandler creates a new flag handler
func NewFlagHandler(evaluator flags.Evaluator, outboxRepo ports.OutboxRepository) *FlagHandler {
	return &FlagHandler{
		evaluator:  evaluator,
		outboxRepo: outboxRepo,
	}
}

// EvaluateFlagRequest represents the request body for flag evaluation
type EvaluateFlagRequest struct {
	SubjectID string `json:"subject_id" validate:"required"`
}

// EvaluateFlagResponse represents the response for flag evaluation
type EvaluateFlagResponse struct {
	FlagKey       string          `json:"flag_key"`
	Enabled       bool            `json:"enabled"`
	Value         json.RawMessage `json:"value"`
	Bucket        int             `json:"bucket"`
	ExperimentKey *string         `json:"experiment_key,omitempty"`
	VariantName   *string         `json:"variant_name,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// EvaluateFlag handles GET /v1/flags/{key}/eval
func (h *FlagHandler) EvaluateFlag(w http.ResponseWriter, r *http.Request) {
	// Set timeout for flag evaluation (200ms as per requirements)
	ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
	defer cancel()

	// Extract flag key from URL path
	vars := mux.Vars(r)
	flagKey := vars["key"]
	
	if flagKey == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_FLAG_KEY", "Flag key is required")
		return
	}

	// Validate flag key format
	if !middleware.ValidateFlagKey(flagKey) {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FLAG_KEY", "Flag key must be alphanumeric with underscores")
		return
	}

	// Get authentication context
	authCtx, ok := middleware.GetAuthContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
		return
	}

	// Extract subject_id from query parameters
	subjectID := r.URL.Query().Get("subject_id")
	if subjectID == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_SUBJECT_ID", "subject_id query parameter is required")
		return
	}

	// Validate subject_id (basic validation - not empty and reasonable length)
	if len(subjectID) == 0 || len(subjectID) > 255 {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_SUBJECT_ID", "subject_id must be between 1 and 255 characters")
		return
	}

	// Create evaluation request
	evalReq := flags.EvaluationRequest{
		TenantID:  authCtx.TenantID,
		FlagKey:   flagKey,
		SubjectID: subjectID,
	}

	// Evaluate the flag
	result, err := h.evaluator.EvaluateFlag(ctx, evalReq)
	if err != nil {
		// Check if it's a timeout error
		if ctx.Err() == context.DeadlineExceeded {
			h.writeErrorResponse(w, http.StatusRequestTimeout, "EVALUATION_TIMEOUT", "Flag evaluation timed out")
			return
		}
		
		// For other errors, return safe default and log the error
		// In production, you'd want proper logging here
		result = &flags.EvaluationResult{
			FlagKey: flagKey,
			Enabled: false,
			Value:   json.RawMessage(`false`),
			Bucket:  0,
		}
	}

	// Publish exposure event to outbox (async, don't block response)
	go h.publishExposureEvent(context.Background(), authCtx.TenantID, result, subjectID)

	// Create response
	response := EvaluateFlagResponse{
		FlagKey:       result.FlagKey,
		Enabled:       result.Enabled,
		Value:         result.Value,
		Bucket:        result.Bucket,
		ExperimentKey: result.ExperimentKey,
		VariantName:   result.VariantName,
	}

	// Write successful response
	h.writeJSONResponse(w, http.StatusOK, response)
}

// publishExposureEvent publishes a flag exposure event to the outbox
func (h *FlagHandler) publishExposureEvent(ctx context.Context, tenantID uuid.UUID, result *flags.EvaluationResult, subjectID string) {
	// Create exposure event payload
	exposurePayload := map[string]interface{}{
		"flag_key":    result.FlagKey,
		"subject_id":  subjectID,
		"enabled":     result.Enabled,
		"value":       result.Value,
		"bucket":      result.Bucket,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	// Add experiment information if present
	if result.ExperimentKey != nil {
		exposurePayload["experiment_key"] = *result.ExperimentKey
	}
	if result.VariantName != nil {
		exposurePayload["variant_name"] = *result.VariantName
	}

	// Marshal payload to JSON
	payloadBytes, err := json.Marshal(exposurePayload)
	if err != nil {
		// Log error in production
		return
	}

	// Create outbox event
	event := &flags.OutboxEvent{
		ID:        uuid.New(),
		TenantID:  tenantID,
		EventType: "flag_exposure",
		Payload:   json.RawMessage(payloadBytes),
		CreatedAt: time.Now(),
	}

	// Add event to outbox (with timeout)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	_ = h.outboxRepo.AddEvent(ctx, event)
	// In production, you'd want to log errors here
}

// writeJSONResponse writes a JSON response
func (h *FlagHandler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, write a simple error
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":{"code":"ENCODING_ERROR","message":"Failed to encode response"}}`)
	}
}

// writeErrorResponse writes a standardized error response
func (h *FlagHandler) writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	
	h.writeJSONResponse(w, status, response)
}