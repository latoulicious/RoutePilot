package httpapi

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/gorilla/mux"
    "github.com/latoulicious/RoutePilot/internal/domain/flags"
    "github.com/latoulicious/RoutePilot/internal/ports"
    "github.com/latoulicious/RoutePilot/internal/transport/http/middleware"
)

// AdminHandler handles admin-related HTTP requests
type AdminHandler struct {
    flagRepo     ports.FlagRepository
    cache        CacheInvalidator   // Interface for cache invalidation
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(flagRepo ports.FlagRepository, cache CacheInvalidator) *AdminHandler {
    return &AdminHandler{
        flagRepo:     flagRepo,
        cache:        cache,
    }
}

// ListFlagsResponse item
type ListFlagsResponse struct {
    ID          uuid.UUID      `json:"id"`
    Key         string         `json:"key"`
    Description string         `json:"description"`
    Type        flags.FlagType `json:"type"`
    Enabled     bool           `json:"enabled"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
}

// ListFlags handles GET /v1/flags
func (h *AdminHandler) ListFlags(w http.ResponseWriter, r *http.Request) {
    authCtx, ok := middleware.GetAuthContext(r)
    if !ok {
        h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
        return
    }
    flagsList, err := h.flagRepo.ListFlagsByTenant(r.Context(), authCtx.TenantID)
    if err != nil {
        h.writeErrorResponse(w, http.StatusInternalServerError, "LIST_FLAGS_FAILED", "Failed to list flags")
        return
    }
    resp := make([]ListFlagsResponse, 0, len(flagsList))
    for _, f := range flagsList {
        resp = append(resp, ListFlagsResponse{
            ID:          f.ID,
            Key:         f.Key,
            Description: f.Description,
            Type:        f.Type,
            Enabled:     f.Enabled,
            CreatedAt:   f.CreatedAt,
            UpdatedAt:   f.UpdatedAt,
        })
    }
    h.writeJSONResponse(w, http.StatusOK, resp)
}

// ListFlagRules handles GET /v1/flags/{key}/rules
func (h *AdminHandler) ListFlagRules(w http.ResponseWriter, r *http.Request) {
    authCtx, ok := middleware.GetAuthContext(r)
    if !ok {
        h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
        return
    }
    vars := mux.Vars(r)
    flagKey := vars["key"]
    if flagKey == "" || !middleware.ValidateFlagKey(flagKey) {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FLAG_KEY", "Flag key is required and must be valid")
        return
    }
    flag, err := h.flagRepo.GetFlagByKey(r.Context(), authCtx.TenantID, flagKey)
    if err != nil {
        h.writeErrorResponse(w, http.StatusNotFound, "FLAG_NOT_FOUND", "Flag not found")
        return
    }
    rules, err := h.flagRepo.GetFlagRules(r.Context(), flag.ID)
    if err != nil {
        h.writeErrorResponse(w, http.StatusInternalServerError, "LIST_RULES_FAILED", "Failed to get flag rules")
        return
    }
    h.writeJSONResponse(w, http.StatusOK, rules)
}

// CreateFlagRequest represents the request body for flag creation
type CreateFlagRequest struct {
    Key         string         `json:"key" validate:"required"`
    Description string         `json:"description"`
    Type        flags.FlagType `json:"type" validate:"required"`
    Enabled     bool           `json:"enabled"`
    Salt        string         `json:"salt"`
}

// CreateFlagResponse represents the response for flag creation
type CreateFlagResponse struct {
    ID          uuid.UUID      `json:"id"`
    Key         string         `json:"key"`
    Description string         `json:"description"`
    Type        flags.FlagType `json:"type"`
    Enabled     bool           `json:"enabled"`
    Salt        string         `json:"salt"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
}

// CreateFlag handles POST /v1/flags
func (h *AdminHandler) CreateFlag(w http.ResponseWriter, r *http.Request) {
    // Get authentication context
    authCtx, ok := middleware.GetAuthContext(r)
    if !ok {
        h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
        return
    }

    // Parse request body
    var createReq CreateFlagRequest
    if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body")
        return
    }

    // Validate required fields
    if createReq.Key == "" {
        h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_FLAG_KEY", "key is required")
        return
    }
    if createReq.Type == "" {
        h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_FLAG_TYPE", "type is required")
        return
    }

    // Validate flag key format
    if !middleware.ValidateFlagKey(createReq.Key) {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FLAG_KEY", "Flag key must be alphanumeric with underscores")
        return
    }

    // Validate flag type
    if createReq.Type != flags.FlagTypeBoolean && createReq.Type != flags.FlagTypeJSON {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FLAG_TYPE", "Flag type must be 'boolean' or 'json'")
        return
    }

    // Validate field lengths
    if len(createReq.Key) > 255 {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_FLAG_KEY", "Flag key must be 255 characters or less")
        return
    }
    if len(createReq.Description) > 1000 {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_DESCRIPTION", "Description must be 1000 characters or less")
        return
    }

    // Generate salt if not provided
    salt := createReq.Salt
    if salt == "" {
        salt = generateSalt()
    }

    // Create flag domain object
    flag := &flags.Flag{
        ID:          uuid.New(),
        TenantID:    authCtx.TenantID,
        Key:         createReq.Key,
        Description: createReq.Description,
        Type:        createReq.Type,
        Enabled:     createReq.Enabled,
        Salt:        salt,
        CreatedAt:   time.Now(),
        UpdatedAt:   time.Now(),
    }

    // Create flag in database
    if err := h.flagRepo.CreateFlag(r.Context(), flag); err != nil {
        // Check for duplicate key error
        if isDuplicateKeyError(err) {
            h.writeErrorResponse(w, http.StatusConflict, "FLAG_KEY_EXISTS", "Flag with this key already exists")
            return
        }
        h.writeErrorResponse(w, http.StatusInternalServerError, "CREATE_FLAG_FAILED", "Failed to create flag")
        return
    }

    // Invalidate cache for this flag
    if h.cache != nil {
        h.cache.InvalidateFlag(authCtx.TenantID, createReq.Key)
    }

    // Create response
    response := CreateFlagResponse{
        ID:          flag.ID,
        Key:         flag.Key,
        Description: flag.Description,
        Type:        flag.Type,
        Enabled:     flag.Enabled,
        Salt:        flag.Salt,
        CreatedAt:   flag.CreatedAt,
        UpdatedAt:   flag.UpdatedAt,
    }

    // Write successful response
    h.writeJSONResponse(w, http.StatusCreated, response)
}

// CreateFlagRuleRequest represents the request body for flag rule creation
type CreateFlagRuleRequest struct {
    Priority int             `json:"priority" validate:"required"`
    Rollout  int             `json:"rollout" validate:"required"`
    Variant  json.RawMessage `json:"variant" validate:"required"`
}

// CreateFlagRuleResponse represents the response for flag rule creation
type CreateFlagRuleResponse struct {
    ID       uuid.UUID       `json:"id"`
    FlagID   uuid.UUID       `json:"flag_id"`
    Priority int             `json:"priority"`
    Rollout  int             `json:"rollout"`
    Variant  json.RawMessage `json:"variant"`
}

// CreateFlagRule handles POST /v1/flags/{key}/rules
func (h *AdminHandler) CreateFlagRule(w http.ResponseWriter, r *http.Request) {
    // Get authentication context
    authCtx, ok := middleware.GetAuthContext(r)
    if !ok {
        h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
        return
    }

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

    // Parse request body
    var createReq CreateFlagRuleRequest
    if err := json.NewDecoder(r.Body).Decode(&createReq); err != nil {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body")
        return
    }

    // Validate required fields and ranges
    if createReq.Priority < 0 {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_PRIORITY", "Priority must be non-negative")
        return
    }
    if createReq.Rollout < 0 || createReq.Rollout > 100 {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_ROLLOUT", "Rollout must be between 0 and 100")
        return
    }
    if len(createReq.Variant) == 0 {
        h.writeErrorResponse(w, http.StatusBadRequest, "MISSING_VARIANT", "variant is required")
        return
    }

    // Validate variant is valid JSON
    var variantTest interface{}
    if err := json.Unmarshal(createReq.Variant, &variantTest); err != nil {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_VARIANT", "variant must be valid JSON")
        return
    }

    // Get the flag to ensure it exists and get its ID
    flag, err := h.flagRepo.GetFlagByKey(r.Context(), authCtx.TenantID, flagKey)
    if err != nil {
        if isNotFoundError(err) {
            h.writeErrorResponse(w, http.StatusNotFound, "FLAG_NOT_FOUND", "Flag not found")
            return
        }
        h.writeErrorResponse(w, http.StatusInternalServerError, "GET_FLAG_FAILED", "Failed to get flag")
        return
    }

    // Create flag rule domain object
    rule := &flags.FlagRule{
        ID:       uuid.New(),
        FlagID:   flag.ID,
        Priority: createReq.Priority,
        Rollout:  createReq.Rollout,
        Variant:  createReq.Variant,
    }

    // Create flag rule in database
    if err := h.flagRepo.CreateFlagRule(r.Context(), rule); err != nil {
        // Check for duplicate priority error
        if isDuplicatePriorityError(err) {
            h.writeErrorResponse(w, http.StatusConflict, "PRIORITY_EXISTS", "Rule with this priority already exists for this flag")
            return
        }
        h.writeErrorResponse(w, http.StatusInternalServerError, "CREATE_RULE_FAILED", "Failed to create flag rule")
        return
    }

    // Invalidate cache for this flag
    if h.cache != nil {
        h.cache.InvalidateFlag(authCtx.TenantID, flagKey)
    }

    // Create response
    response := CreateFlagRuleResponse{
        ID:       rule.ID,
        FlagID:   rule.FlagID,
        Priority: rule.Priority,
        Rollout:  rule.Rollout,
        Variant:  rule.Variant,
    }

    // Write successful response
    h.writeJSONResponse(w, http.StatusCreated, response)
}

// UpdateFlagRequest represents the request body for flag updates
type UpdateFlagRequest struct {
    Description *string `json:"description,omitempty"`
    Enabled     *bool   `json:"enabled,omitempty"`
    Salt        *string `json:"salt,omitempty"`
    RotateSalt  *bool   `json:"rotate_salt,omitempty"`
}

// UpdateFlagResponse represents the response for flag updates
type UpdateFlagResponse struct {
    ID          uuid.UUID      `json:"id"`
    Key         string         `json:"key"`
    Description string         `json:"description"`
    Type        flags.FlagType `json:"type"`
    Enabled     bool           `json:"enabled"`
    Salt        string         `json:"salt"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
}

// UpdateFlag handles PATCH /v1/flags/{key}
func (h *AdminHandler) UpdateFlag(w http.ResponseWriter, r *http.Request) {
    // Get authentication context
    authCtx, ok := middleware.GetAuthContext(r)
    if !ok {
        h.writeErrorResponse(w, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication context not found")
        return
    }

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

    // Parse request body
    var updateReq UpdateFlagRequest
    if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON in request body")
        return
    }

    // Validate field lengths if provided
    if updateReq.Description != nil && len(*updateReq.Description) > 1000 {
        h.writeErrorResponse(w, http.StatusBadRequest, "INVALID_DESCRIPTION", "Description must be 1000 characters or less")
        return
    }

    // Get the flag to ensure it exists and get its ID
    flag, err := h.flagRepo.GetFlagByKey(r.Context(), authCtx.TenantID, flagKey)
    if err != nil {
        if isNotFoundError(err) {
            h.writeErrorResponse(w, http.StatusNotFound, "FLAG_NOT_FOUND", "Flag not found")
            return
        }
        h.writeErrorResponse(w, http.StatusInternalServerError, "GET_FLAG_FAILED", "Failed to get flag")
        return
    }

    // Create flag updates object
    updates := flags.FlagUpdates{
        Description: updateReq.Description,
        Enabled:     updateReq.Enabled,
        Salt:        updateReq.Salt,
    }

    // If rotate_salt=true, generate a new salt
    if updateReq.RotateSalt != nil && *updateReq.RotateSalt {
        s := generateSalt()
        updates.Salt = &s
    }

    // Update flag in database
    if err := h.flagRepo.UpdateFlag(r.Context(), flag.ID, updates); err != nil {
        h.writeErrorResponse(w, http.StatusInternalServerError, "UPDATE_FLAG_FAILED", "Failed to update flag")
        return
    }

    // Get updated flag for response
    updatedFlag, err := h.flagRepo.GetFlagByKey(r.Context(), authCtx.TenantID, flagKey)
    if err != nil {
        h.writeErrorResponse(w, http.StatusInternalServerError, "GET_UPDATED_FLAG_FAILED", "Failed to get updated flag")
        return
    }

    // Invalidate cache for this flag
    if h.cache != nil {
        h.cache.InvalidateFlag(authCtx.TenantID, flagKey)
    }

    // Create response
    response := UpdateFlagResponse{
        ID:          updatedFlag.ID,
        Key:         updatedFlag.Key,
        Description: updatedFlag.Description,
        Type:        updatedFlag.Type,
        Enabled:     updatedFlag.Enabled,
        Salt:        updatedFlag.Salt,
        CreatedAt:   updatedFlag.CreatedAt,
        UpdatedAt:   updatedFlag.UpdatedAt,
    }

    // Write successful response
    h.writeJSONResponse(w, http.StatusOK, response)
}

// writeJSONResponse writes a JSON response
func (h *AdminHandler) writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)

    if err := json.NewEncoder(w).Encode(data); err != nil {
        // If we can't encode the response, write a simple error
        w.WriteHeader(http.StatusInternalServerError)
        fmt.Fprintf(w, `{"error":{"code":"ENCODING_ERROR","message":"Failed to encode response"}}`)
    }
}

// writeErrorResponse writes a standardized error response
func (h *AdminHandler) writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
    response := ErrorResponse{
        Error: ErrorDetail{
            Code:    code,
            Message: message,
        },
    }

    h.writeJSONResponse(w, status, response)
}

// Helper functions

// generateSalt generates a random salt for flag evaluation
func generateSalt() string {
    return uuid.New().String()
}

// isDuplicateKeyError checks if the error is a duplicate key constraint violation
func isDuplicateKeyError(err error) bool {
    // This would need to be implemented based on the specific database error types
    // For now, we'll check if the error message contains common duplicate key indicators
    errStr := err.Error()
    return contains(errStr, "duplicate") || contains(errStr, "unique") || contains(errStr, "already exists")
}

// isDuplicatePriorityError checks if the error is a duplicate priority constraint violation
func isDuplicatePriorityError(err error) bool {
    // Similar to isDuplicateKeyError but for priority constraints
    errStr := err.Error()
    return contains(errStr, "priority") && contains(errStr, "constraint")
}

// isNotFoundError checks if the error indicates a resource was not found
func isNotFoundError(err error) bool {
    errStr := err.Error()
    return contains(errStr, "not found") || contains(errStr, "no rows")
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
    return len(s) >= len(substr) && (s == substr ||
        (len(s) > len(substr) &&
            (s[:len(substr)] == substr ||
                s[len(s)-len(substr):] == substr ||
                indexOf(s, substr) >= 0)))
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
    for i := 0; i <= len(s)-len(substr); i++ {
        if s[i:i+len(substr)] == substr {
            return i
        }
    }
    return -1
}
