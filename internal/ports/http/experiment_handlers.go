package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

// ExperimentHandler handles experiment-related HTTP requests
type ExperimentHandler struct {
	experimentRepo ports.ExperimentRepository
	flagRepo       ports.FlagRepository
	cache          CacheInvalidator
}

// NewExperimentHandler creates a new experiment handler
func NewExperimentHandler(experimentRepo ports.ExperimentRepository, flagRepo ports.FlagRepository, cache CacheInvalidator) *ExperimentHandler {
	return &ExperimentHandler{
		experimentRepo: experimentRepo,
		flagRepo:       flagRepo,
		cache:          cache,
	}
}

// CreateExperimentRequest represents the request body for experiment creation
type CreateExperimentRequest struct {
	Key     string `json:"key" validate:"required"`
	FlagKey string `json:"flag_key" validate:"required"`
	Traffic int    `json:"traffic" validate:"required"`
}

// CreateExperimentResponse represents the response for experiment creation
type CreateExperimentResponse struct {
	ID       uuid.UUID                `json:"id"`
	Key      string                   `json:"key"`
	FlagKey  string                   `json:"flag_key"`
	Status   flags.ExperimentStatus   `json:"status"`
	Traffic  int                      `json:"traffic"`
	Variants []*flags.ExperimentVariant `json:"variants"`
}

// CreateExperiment handles POST /v1/experiments
func (h *ExperimentHandler) CreateExperiment(w http.ResponseWriter, r *http.Request) {
	// Get authentication context
	authCtx, ok := middleware.GetAuthContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
		return
	}

	// Parse request body
	var createReq CreateExperimentRequest
	if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body")
		return
	}

	// Validate required fields
	if createReq.Key == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_EXPERIMENT_KEY", "key is required")
		return
	}
	if createReq.FlagKey == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_FLAG_KEY", "flag_key is required")
		return
	}

	// Validate experiment key format
	if !middleware.ValidateFlagKey(createReq.Key) {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_EXPERIMENT_KEY", "Experiment key must be alphanumeric with underscores")
		return
	}

	// Validate flag key format
	if !middleware.ValidateFlagKey(createReq.FlagKey) {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FLAG_KEY", "Flag key must be alphanumeric with underscores")
		return
	}

	// Validate traffic range
	if createReq.Traffic < 0 || createReq.Traffic > 100 {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_TRAFFIC", "Traffic must be between 0 and 100")
		return
	}

	// Validate field lengths
	if len(createReq.Key) > 255 {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_EXPERIMENT_KEY", "Experiment key must be 255 characters or less")
		return
	}

	// Get the flag to ensure it exists and get its ID
	flag, err := h.flagRepo.GetFlagByKey(r.Context(), authCtx.TenantID, createReq.FlagKey)
	if err != nil {
		if isNotFoundError(err) {
			h.writeErrorResponse(w, http.StatusNotFound, "FLAG_NOT_FOUND", "Flag not found")
			return
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "GET_FLAG_FAILED", "Failed to get flag")
		return
	}

	// Create experiment domain object
	experiment := &flags.Experiment{
		ID:       uuid.New(),
		TenantID: authCtx.TenantID,
		Key:      createReq.Key,
		FlagID:   flag.ID,
		Status:   flags.ExperimentStatusDraft,
		Traffic:  createReq.Traffic,
		Variants: []*flags.ExperimentVariant{},
	}

	// Create experiment in database
	if err := h.experimentRepo.CreateExperiment(r.Context(), experiment); err != nil {
		// Check for duplicate key error
		if isDuplicateKeyError(err) {
			h.writeErrorResponse(w, http.StatusConflict, "EXPERIMENT_KEY_EXISTS", "Experiment with this key already exists")
			return
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "CREATE_EXPERIMENT_FAILED", "Failed to create experiment")
		return
	}

	// Create response
	response := CreateExperimentResponse{
		ID:       experiment.ID,
		Key:      experiment.Key,
		FlagKey:  flag.Key,
		Status:   experiment.Status,
		Traffic:  experiment.Traffic,
		Variants: experiment.Variants,
	}

	// Write successful response
	h.writeJSONResponse(w, http.StatusCreated, response)
}

// CreateVariantRequest represents the request body for variant creation
type CreateVariantRequest struct {
	Name   string          `json:"name" validate:"required"`
	Weight int             `json:"weight" validate:"required"`
	Value  json.RawMessage `json:"value" validate:"required"`
}

// CreateVariantResponse represents the response for variant creation
type CreateVariantResponse struct {
	ID           uuid.UUID       `json:"id"`
	ExperimentID uuid.UUID       `json:"experiment_id"`
	Name         string          `json:"name"`
	Weight       int             `json:"weight"`
	Value        json.RawMessage `json:"value"`
}

// CreateVariant handles POST /v1/experiments/{key}/variants
func (h *ExperimentHandler) CreateVariant(w http.ResponseWriter, r *http.Request) {
	// Get authentication context
	authCtx, ok := middleware.GetAuthContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
		return
	}

	// Extract experiment key from URL path
	vars := mux.Vars(r)
	experimentKey := vars["key"]
	
	if experimentKey == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_EXPERIMENT_KEY", "Experiment key is required")
		return
	}

	// Validate experiment key format
	if !middleware.ValidateFlagKey(experimentKey) {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_EXPERIMENT_KEY", "Experiment key must be alphanumeric with underscores")
		return
	}

	// Parse request body
	var createReq CreateVariantRequest
	if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body")
		return
	}

	// Validate required fields
	if createReq.Name == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_VARIANT_NAME", "name is required")
		return
	}
	if createReq.Weight < 0 || createReq.Weight > 100 {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_WEIGHT", "Weight must be between 0 and 100")
		return
	}
	if createReq.Value == nil || len(createReq.Value) == 0 {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_VALUE", "value is required")
		return
	}

	// Validate value is valid JSON
	var valueTest interface{}
	if err := json.Unmarshal(createReq.Value, &valueTest); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_VALUE", "value must be valid JSON")
		return
	}

	// Validate field lengths
	if len(createReq.Name) > 255 {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_VARIANT_NAME", "Variant name must be 255 characters or less")
		return
	}

	// Get the experiment to ensure it exists and get its ID
	experiment, err := h.experimentRepo.GetExperimentByTenantKey(r.Context(), authCtx.TenantID, experimentKey)
	if err != nil {
		if isNotFoundError(err) {
			h.writeErrorResponse(w, http.StatusNotFound, "EXPERIMENT_NOT_FOUND", "Experiment not found")
			return
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "GET_EXPERIMENT_FAILED", "Failed to get experiment")
		return
	}

	// Create variant domain object
	variant := &flags.ExperimentVariant{
		ID:           uuid.New(),
		ExperimentID: experiment.ID,
		Name:         createReq.Name,
		Weight:       createReq.Weight,
		Value:        createReq.Value,
	}

	// Create variant in database
	if err := h.experimentRepo.CreateExperimentVariant(r.Context(), variant); err != nil {
		// Check for duplicate name error
		if isDuplicateKeyError(err) {
			h.writeErrorResponse(w, http.StatusConflict, "VARIANT_NAME_EXISTS", "Variant with this name already exists for this experiment")
			return
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "CREATE_VARIANT_FAILED", "Failed to create variant")
		return
	}

	// Create response
	response := CreateVariantResponse{
		ID:           variant.ID,
		ExperimentID: variant.ExperimentID,
		Name:         variant.Name,
		Weight:       variant.Weight,
		Value:        variant.Value,
	}

	// Write successful response
	h.writeJSONResponse(w, http.StatusCreated, response)
}

// UpdateExperimentRequest represents the request body for experiment updates
type UpdateExperimentRequest struct {
	Status *flags.ExperimentStatus `json:"status,omitempty"`
}

// UpdateExperimentResponse represents the response for experiment updates
type UpdateExperimentResponse struct {
	ID       uuid.UUID                `json:"id"`
	Key      string                   `json:"key"`
	Status   flags.ExperimentStatus   `json:"status"`
	Traffic  int                      `json:"traffic"`
	Variants []*flags.ExperimentVariant `json:"variants"`
}

// UpdateExperiment handles PATCH /v1/experiments/{key}
func (h *ExperimentHandler) UpdateExperiment(w http.ResponseWriter, r *http.Request) {
	// Get authentication context
	authCtx, ok := middleware.GetAuthContext(r)
	if !ok {
		h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
		return
	}

	// Extract experiment key from URL path
	vars := mux.Vars(r)
	experimentKey := vars["key"]
	
	if experimentKey == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_EXPERIMENT_KEY", "Experiment key is required")
		return
	}

	// Validate experiment key format
	if !middleware.ValidateFlagKey(experimentKey) {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_EXPERIMENT_KEY", "Experiment key must be alphanumeric with underscores")
		return
	}

	// Parse request body
	var updateReq UpdateExperimentRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body")
		return
	}

	// Validate status if provided
	if updateReq.Status != nil {
		validStatuses := []flags.ExperimentStatus{
			flags.ExperimentStatusDraft,
			flags.ExperimentStatusRunning,
			flags.ExperimentStatusPaused,
			flags.ExperimentStatusStopped,
		}
		
		isValid := false
		for _, validStatus := range validStatuses {
			if *updateReq.Status == validStatus {
				isValid = true
				break
			}
		}
		
		if !isValid {
			h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_STATUS", "Status must be one of: draft, running, paused, stopped")
			return
		}
	}

	// Get the experiment to ensure it exists
	experiment, err := h.experimentRepo.GetExperimentByTenantKey(r.Context(), authCtx.TenantID, experimentKey)
	if err != nil {
		if isNotFoundError(err) {
			h.writeErrorResponse(w, http.StatusNotFound, "EXPERIMENT_NOT_FOUND", "Experiment not found")
			return
		}
		h.writeErrorResponse(w, http.StatusInternalServerError, "GET_EXPERIMENT_FAILED", "Failed to get experiment")
		return
	}

	// Update experiment status if provided
	if updateReq.Status != nil {
		if err := h.experimentRepo.UpdateExperiment(r.Context(), experiment.ID, *updateReq.Status); err != nil {
			h.writeErrorResponse(w, http.StatusInternalServerError, "UPDATE_EXPERIMENT_FAILED", "Failed to update experiment")
			return
		}
		experiment.Status = *updateReq.Status
	}

	// Invalidate cache for the associated flag if status changed
	if updateReq.Status != nil && h.cache != nil {
		// Get the flag to invalidate cache
		flag, err := h.flagRepo.GetFlagByID(r.Context(), experiment.FlagID)
		if err == nil {
			h.cache.InvalidateFlag(authCtx.TenantID, flag.Key)
		}
	}

	// Create response
	response := UpdateExperimentResponse{
		ID:       experiment.ID,
		Key:      experiment.Key,
		Status:   experiment.Status,
		Traffic:  experiment.Traffic,
		Variants: experiment.Variants,
	}

	// Write successful response
	h.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response
func (h *ExperimentHandler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, write a simple error
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":{"code":"ENCODING_ERROR","message":"Failed to encode response"}}`)
	}
}

// writeErrorResponse writes a standardized error response
func (h *ExperimentHandler) writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	
	h.writeJSONResponse(w, status, response)
}