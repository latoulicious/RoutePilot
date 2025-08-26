package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

// Mock implementations for testing admin handlers
type mockFlagRepo struct {
	flags       map[string]*flags.Flag
	createError error
	updateError error
	getError    error
}

func (m *mockFlagRepo) GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Flag, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	
	flagKey := fmt.Sprintf("%s:%s", tenantID.String(), key)
	if flag, exists := m.flags[flagKey]; exists {
		return flag, nil
	}
	return nil, fmt.Errorf("flag not found")
}

func (m *mockFlagRepo) GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*flags.FlagRule, error) {
	return nil, nil
}

func (m *mockFlagRepo) CreateFlag(ctx context.Context, flag *flags.Flag) error {
	if m.createError != nil {
		return m.createError
	}
	
	flagKey := fmt.Sprintf("%s:%s", flag.TenantID.String(), flag.Key)
	if _, exists := m.flags[flagKey]; exists {
		return fmt.Errorf("duplicate key constraint violation")
	}
	
	// Simulate database behavior - set timestamps and ID
	flag.ID = uuid.New()
	flag.CreatedAt = time.Now()
	flag.UpdatedAt = time.Now()
	
	m.flags[flagKey] = flag
	return nil
}

func (m *mockFlagRepo) UpdateFlag(ctx context.Context, flagID uuid.UUID, updates flags.FlagUpdates) error {
	if m.updateError != nil {
		return m.updateError
	}
	
	// Find flag by ID
	for _, flag := range m.flags {
		if flag.ID == flagID {
			// Apply updates
			if updates.Description != nil {
				flag.Description = *updates.Description
			}
			if updates.Enabled != nil {
				flag.Enabled = *updates.Enabled
			}
			if updates.Salt != nil {
				flag.Salt = *updates.Salt
			}
			flag.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("flag not found")
}

func (m *mockFlagRepo) GetFlagByID(ctx context.Context, flagID uuid.UUID) (*flags.Flag, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	
	// For testing, we'll just return a mock flag
	for _, flag := range m.flags {
		if flag.ID == flagID {
			return flag, nil
		}
	}
	return nil, fmt.Errorf("flag not found")
}

func (m *mockFlagRepo) DeleteFlag(ctx context.Context, flagID uuid.UUID) error {
	return fmt.Errorf("not implemented")
}

type mockFlagRuleRepo struct {
	rules       []*flags.FlagRule
	createError error
}

func (m *mockFlagRuleRepo) CreateFlagRule(ctx context.Context, rule *flags.FlagRule) error {
	if m.createError != nil {
		return m.createError
	}
	
	// Check for duplicate priority
	for _, existingRule := range m.rules {
		if existingRule.FlagID == rule.FlagID && existingRule.Priority == rule.Priority {
			return fmt.Errorf("priority constraint violation")
		}
	}
	
	// Simulate database behavior - set ID
	rule.ID = uuid.New()
	m.rules = append(m.rules, rule)
	return nil
}

type mockCache struct {
	invalidatedFlags []string
}

func (m *mockCache) InvalidateFlag(tenantID uuid.UUID, flagKey string) {
	m.invalidatedFlags = append(m.invalidatedFlags, fmt.Sprintf("%s:%s", tenantID.String(), flagKey))
}

func TestAdminHandler_CreateFlag(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		repoError      error
		expectedStatus int
		expectError    bool
		expectedErrorCode string
	}{
		{
			name: "successful flag creation - boolean",
			requestBody: CreateFlagRequest{
				Key:         "test_flag",
				Description: "Test flag for unit tests",
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
				Salt:        "custom_salt",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "successful flag creation - json",
			requestBody: CreateFlagRequest{
				Key:         "json_flag",
				Description: "JSON flag for testing",
				Type:        flags.FlagTypeJSON,
				Enabled:     false,
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "missing key",
			requestBody: CreateFlagRequest{
				Description: "Flag without key",
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_FLAG_KEY",
		},
		{
			name: "missing type",
			requestBody: CreateFlagRequest{
				Key:         "test_flag",
				Description: "Flag without type",
				Enabled:     true,
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_FLAG_TYPE",
		},
		{
			name: "invalid flag key format",
			requestBody: CreateFlagRequest{
				Key:         "invalid-flag!",
				Description: "Flag with invalid key",
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_FLAG_KEY",
		},
		{
			name: "invalid flag type",
			requestBody: CreateFlagRequest{
				Key:         "test_flag",
				Description: "Flag with invalid type",
				Type:        "invalid_type",
				Enabled:     true,
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_FLAG_TYPE",
		},
		{
			name: "key too long",
			requestBody: CreateFlagRequest{
				Key:         string(make([]byte, 256)), // 256 characters
				Description: "Flag with long key",
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_FLAG_KEY",
		},
		{
			name: "description too long",
			requestBody: CreateFlagRequest{
				Key:         "test_flag",
				Description: string(make([]byte, 1001)), // 1001 characters
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_DESCRIPTION",
		},
		{
			name: "duplicate flag key",
			requestBody: CreateFlagRequest{
				Key:         "test_flag",
				Description: "Duplicate flag",
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
			},
			repoError:         fmt.Errorf("duplicate key constraint violation"),
			expectedStatus:    http.StatusConflict,
			expectError:       true,
			expectedErrorCode: "FLAG_KEY_EXISTS",
		},
		{
			name: "database error",
			requestBody: CreateFlagRequest{
				Key:         "test_flag",
				Description: "Flag that causes DB error",
				Type:        flags.FlagTypeBoolean,
				Enabled:     true,
			},
			repoError:         fmt.Errorf("database connection failed"),
			expectedStatus:    http.StatusInternalServerError,
			expectError:       true,
			expectedErrorCode: "CREATE_FLAG_FAILED",
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			expectedErrorCode: "INVALID_JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock dependencies
			mockFlagRepo := &mockFlagRepo{
				flags:       make(map[string]*flags.Flag),
				createError: tt.repoError,
			}
			mockRuleRepo := &mockFlagRuleRepo{}
			mockCache := &mockCache{}

			// Create handler
			handler := NewAdminHandler(mockFlagRepo, mockRuleRepo, mockCache)

			// Create request body
			var reqBody []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			// Create request
			req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// Add auth context to request
			tenantID := uuid.New()
			authCtx := &middleware.AuthContext{
				TenantID: tenantID,
				APIKeyID: uuid.New(),
			}
			ctx := context.WithValue(req.Context(), "auth", authCtx)
			req = req.WithContext(ctx)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.CreateFlag(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check response
			if tt.expectError {
				var errorResp ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&errorResp); err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}
				if errorResp.Error.Code != tt.expectedErrorCode {
					t.Errorf("Expected error code %s, got %s", tt.expectedErrorCode, errorResp.Error.Code)
				}
			} else {
				var response CreateFlagResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Verify response fields
				createReq := tt.requestBody.(CreateFlagRequest)
				if response.Key != createReq.Key {
					t.Errorf("Expected key %s, got %s", createReq.Key, response.Key)
				}
				if response.Description != createReq.Description {
					t.Errorf("Expected description %s, got %s", createReq.Description, response.Description)
				}
				if response.Type != createReq.Type {
					t.Errorf("Expected type %s, got %s", createReq.Type, response.Type)
				}
				if response.Enabled != createReq.Enabled {
					t.Errorf("Expected enabled %v, got %v", createReq.Enabled, response.Enabled)
				}

				// Verify salt is set (either custom or generated)
				if response.Salt == "" {
					t.Error("Expected salt to be set")
				}
				if createReq.Salt != "" && response.Salt != createReq.Salt {
					t.Errorf("Expected salt %s, got %s", createReq.Salt, response.Salt)
				}

				// Verify timestamps are set
				if response.CreatedAt.IsZero() {
					t.Error("Expected created_at to be set")
				}
				if response.UpdatedAt.IsZero() {
					t.Error("Expected updated_at to be set")
				}

				// Verify cache invalidation was called
				expectedCacheKey := fmt.Sprintf("%s:%s", tenantID.String(), createReq.Key)
				found := false
				for _, invalidated := range mockCache.invalidatedFlags {
					if invalidated == expectedCacheKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected cache invalidation for %s", expectedCacheKey)
				}
			}
		})
	}
}

func TestAdminHandler_CreateFlagRule(t *testing.T) {
	// Pre-create a flag for testing
	tenantID := uuid.New()
	flagID := uuid.New()
	testFlag := &flags.Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test_flag",
		Type:     flags.FlagTypeBoolean,
		Enabled:  true,
		Salt:     "test_salt",
	}

	tests := []struct {
		name           string
		flagKey        string
		requestBody    interface{}
		flagExists     bool
		repoError      error
		expectedStatus int
		expectError    bool
		expectedErrorCode string
	}{
		{
			name:    "successful rule creation",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:     true,
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:    "successful rule creation with complex variant",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 2,
				Rollout:  75,
				Variant:  json.RawMessage(`{"color": "blue", "size": "large", "features": ["a", "b"]}`),
			},
			flagExists:     true,
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:    "missing flag key in URL",
			flagKey: "",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_FLAG_KEY",
		},
		{
			name:    "invalid flag key format",
			flagKey: "invalid-flag!",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_FLAG_KEY",
		},
		{
			name:    "negative priority",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: -1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:        true,
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_PRIORITY",
		},
		{
			name:    "rollout below range",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  -1,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:        true,
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_ROLLOUT",
		},
		{
			name:    "rollout above range",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  101,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:        true,
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_ROLLOUT",
		},
		{
			name:    "missing variant",
			flagKey: "test_flag",
			requestBody: map[string]interface{}{
				"priority": 1,
				"rollout":  50,
			},
			flagExists:        true,
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_VARIANT",
		},
		{
			name:    "invalid variant JSON",
			flagKey: "test_flag",
			requestBody: `{"priority": 1, "rollout": 50, "variant": {invalid: "json"}}`,
			flagExists:        true,
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_JSON",
		},
		{
			name:    "flag not found",
			flagKey: "nonexistent_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:        false,
			expectedStatus:    http.StatusNotFound,
			expectError:       true,
			expectedErrorCode: "FLAG_NOT_FOUND",
		},
		{
			name:    "duplicate priority",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:        true,
			repoError:         fmt.Errorf("priority constraint violation"),
			expectedStatus:    http.StatusConflict,
			expectError:       true,
			expectedErrorCode: "PRIORITY_EXISTS",
		},
		{
			name:    "database error",
			flagKey: "test_flag",
			requestBody: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			flagExists:        true,
			repoError:         fmt.Errorf("database connection failed"),
			expectedStatus:    http.StatusInternalServerError,
			expectError:       true,
			expectedErrorCode: "CREATE_RULE_FAILED",
		},
		{
			name:    "invalid JSON body",
			flagKey: "test_flag",
			requestBody: "invalid json",
			flagExists:     true,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			expectedErrorCode: "INVALID_JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock dependencies
			mockFlagRepo := &mockFlagRepo{
				flags: make(map[string]*flags.Flag),
			}
			
			// Add test flag if it should exist
			if tt.flagExists {
				flagKey := fmt.Sprintf("%s:%s", tenantID.String(), tt.flagKey)
				mockFlagRepo.flags[flagKey] = testFlag
			}

			mockRuleRepo := &mockFlagRuleRepo{
				createError: tt.repoError,
			}
			mockCache := &mockCache{}

			// Create handler
			handler := NewAdminHandler(mockFlagRepo, mockRuleRepo, mockCache)

			// Create request body
			var reqBody []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			// Create request
			req := httptest.NewRequest("POST", "/v1/flags/"+tt.flagKey+"/rules", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// Add auth context to request
			authCtx := &middleware.AuthContext{
				TenantID: tenantID,
				APIKeyID: uuid.New(),
			}
			ctx := context.WithValue(req.Context(), "auth", authCtx)
			req = req.WithContext(ctx)

			// Add mux vars
			req = mux.SetURLVars(req, map[string]string{"key": tt.flagKey})

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.CreateFlagRule(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check response
			if tt.expectError {
				var errorResp ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&errorResp); err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}
				if errorResp.Error.Code != tt.expectedErrorCode {
					t.Errorf("Expected error code %s, got %s", tt.expectedErrorCode, errorResp.Error.Code)
				}
			} else {
				var response CreateFlagRuleResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Verify response fields
				createReq := tt.requestBody.(CreateFlagRuleRequest)
				if response.FlagID != flagID {
					t.Errorf("Expected flag_id %s, got %s", flagID, response.FlagID)
				}
				if response.Priority != createReq.Priority {
					t.Errorf("Expected priority %d, got %d", createReq.Priority, response.Priority)
				}
				if response.Rollout != createReq.Rollout {
					t.Errorf("Expected rollout %d, got %d", createReq.Rollout, response.Rollout)
				}
				// Compare JSON variants by unmarshaling and comparing
				var expectedVariant, actualVariant interface{}
				if createReq, ok := tt.requestBody.(CreateFlagRuleRequest); ok {
					if err := json.Unmarshal(createReq.Variant, &expectedVariant); err != nil {
						t.Errorf("Failed to unmarshal expected variant: %v", err)
					}
					if err := json.Unmarshal(response.Variant, &actualVariant); err != nil {
						t.Errorf("Failed to unmarshal actual variant: %v", err)
					}
					expectedJSON, _ := json.Marshal(expectedVariant)
					actualJSON, _ := json.Marshal(actualVariant)
					if string(expectedJSON) != string(actualJSON) {
						t.Errorf("Expected variant %s, got %s", expectedJSON, actualJSON)
					}
				}

				// Verify cache invalidation was called
				expectedCacheKey := fmt.Sprintf("%s:%s", tenantID.String(), tt.flagKey)
				found := false
				for _, invalidated := range mockCache.invalidatedFlags {
					if invalidated == expectedCacheKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected cache invalidation for %s", expectedCacheKey)
				}
			}
		})
	}
}

func TestAdminHandler_UpdateFlag(t *testing.T) {
	// Pre-create a flag for testing
	tenantID := uuid.New()
	flagID := uuid.New()
	testFlag := &flags.Flag{
		ID:          flagID,
		TenantID:    tenantID,
		Key:         "test_flag",
		Description: "Original description",
		Type:        flags.FlagTypeBoolean,
		Enabled:     false,
		Salt:        "original_salt",
		CreatedAt:   time.Now().Add(-1 * time.Hour),
		UpdatedAt:   time.Now().Add(-1 * time.Hour),
	}

	tests := []struct {
		name           string
		flagKey        string
		requestBody    interface{}
		flagExists     bool
		repoError      error
		expectedStatus int
		expectError    bool
		expectedErrorCode string
	}{
		{
			name:    "successful flag update - all fields",
			flagKey: "test_flag",
			requestBody: UpdateFlagRequest{
				Description: stringPtr("Updated description"),
				Enabled:     boolPtr(true),
				Salt:        stringPtr("new_salt"),
			},
			flagExists:     true,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:    "successful flag update - enabled only",
			flagKey: "test_flag",
			requestBody: UpdateFlagRequest{
				Enabled: boolPtr(true),
			},
			flagExists:     true,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:    "successful flag update - description only",
			flagKey: "test_flag",
			requestBody: UpdateFlagRequest{
				Description: stringPtr("New description"),
			},
			flagExists:     true,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:    "successful flag update - salt only",
			flagKey: "test_flag",
			requestBody: UpdateFlagRequest{
				Salt: stringPtr("rotated_salt"),
			},
			flagExists:     true,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:    "missing flag key in URL",
			flagKey: "",
			requestBody: UpdateFlagRequest{
				Enabled: boolPtr(true),
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_FLAG_KEY",
		},
		{
			name:    "invalid flag key format",
			flagKey: "invalid-flag!",
			requestBody: UpdateFlagRequest{
				Enabled: boolPtr(true),
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_FLAG_KEY",
		},
		{
			name:    "description too long",
			flagKey: "test_flag",
			requestBody: UpdateFlagRequest{
				Description: stringPtr(string(make([]byte, 1001))), // 1001 characters
			},
			flagExists:        true,
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_DESCRIPTION",
		},
		{
			name:    "flag not found",
			flagKey: "nonexistent_flag",
			requestBody: UpdateFlagRequest{
				Enabled: boolPtr(true),
			},
			flagExists:        false,
			expectedStatus:    http.StatusNotFound,
			expectError:       true,
			expectedErrorCode: "FLAG_NOT_FOUND",
		},
		{
			name:    "database error on update",
			flagKey: "test_flag",
			requestBody: UpdateFlagRequest{
				Enabled: boolPtr(true),
			},
			flagExists:        true,
			repoError:         fmt.Errorf("database connection failed"),
			expectedStatus:    http.StatusInternalServerError,
			expectError:       true,
			expectedErrorCode: "UPDATE_FLAG_FAILED",
		},
		{
			name:    "invalid JSON body",
			flagKey: "test_flag",
			requestBody: "invalid json",
			flagExists:     true,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
			expectedErrorCode: "INVALID_JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock dependencies
			mockFlagRepo := &mockFlagRepo{
				flags:       make(map[string]*flags.Flag),
				updateError: tt.repoError,
			}
			
			// Add test flag if it should exist
			if tt.flagExists {
				flagKey := fmt.Sprintf("%s:%s", tenantID.String(), tt.flagKey)
				// Create a copy of the test flag to avoid modifying the original
				flagCopy := *testFlag
				mockFlagRepo.flags[flagKey] = &flagCopy
			}

			mockRuleRepo := &mockFlagRuleRepo{}
			mockCache := &mockCache{}

			// Create handler
			handler := NewAdminHandler(mockFlagRepo, mockRuleRepo, mockCache)

			// Create request body
			var reqBody []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("Failed to marshal request body: %v", err)
				}
			}

			// Create request
			req := httptest.NewRequest("PATCH", "/v1/flags/"+tt.flagKey, bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// Add auth context to request
			authCtx := &middleware.AuthContext{
				TenantID: tenantID,
				APIKeyID: uuid.New(),
			}
			ctx := context.WithValue(req.Context(), "auth", authCtx)
			req = req.WithContext(ctx)

			// Add mux vars
			req = mux.SetURLVars(req, map[string]string{"key": tt.flagKey})

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.UpdateFlag(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Check response
			if tt.expectError {
				var errorResp ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&errorResp); err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}
				if errorResp.Error.Code != tt.expectedErrorCode {
					t.Errorf("Expected error code %s, got %s", tt.expectedErrorCode, errorResp.Error.Code)
				}
			} else {
				var response UpdateFlagResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Verify response fields
				if response.ID != flagID {
					t.Errorf("Expected ID %s, got %s", flagID, response.ID)
				}
				if response.Key != tt.flagKey {
					t.Errorf("Expected key %s, got %s", tt.flagKey, response.Key)
				}
				if response.Type != testFlag.Type {
					t.Errorf("Expected type %s, got %s", testFlag.Type, response.Type)
				}

				// Verify updated fields
				updateReq := tt.requestBody.(UpdateFlagRequest)
				if updateReq.Description != nil {
					if response.Description != *updateReq.Description {
						t.Errorf("Expected description %s, got %s", *updateReq.Description, response.Description)
					}
				}
				if updateReq.Enabled != nil {
					if response.Enabled != *updateReq.Enabled {
						t.Errorf("Expected enabled %v, got %v", *updateReq.Enabled, response.Enabled)
					}
				}
				if updateReq.Salt != nil {
					if response.Salt != *updateReq.Salt {
						t.Errorf("Expected salt %s, got %s", *updateReq.Salt, response.Salt)
					}
				}

				// Verify timestamps
				if response.CreatedAt.IsZero() {
					t.Error("Expected created_at to be set")
				}
				if response.UpdatedAt.IsZero() {
					t.Error("Expected updated_at to be set")
				}

				// Verify cache invalidation was called
				expectedCacheKey := fmt.Sprintf("%s:%s", tenantID.String(), tt.flagKey)
				found := false
				for _, invalidated := range mockCache.invalidatedFlags {
					if invalidated == expectedCacheKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected cache invalidation for %s", expectedCacheKey)
				}
			}
		})
	}
}

func TestAdminHandler_NoAuthContext(t *testing.T) {
	// Create mock dependencies
	mockFlagRepo := &mockFlagRepo{flags: make(map[string]*flags.Flag)}
	mockRuleRepo := &mockFlagRuleRepo{}
	mockCache := &mockCache{}

	// Create handler
	handler := NewAdminHandler(mockFlagRepo, mockRuleRepo, mockCache)

	tests := []struct {
		name    string
		method  string
		path    string
		body    interface{}
		handler http.HandlerFunc
	}{
		{
			name:   "CreateFlag without auth",
			method: "POST",
			path:   "/v1/flags",
			body: CreateFlagRequest{
				Key:  "test_flag",
				Type: flags.FlagTypeBoolean,
			},
			handler: handler.CreateFlag,
		},
		{
			name:   "CreateFlagRule without auth",
			method: "POST",
			path:   "/v1/flags/test_flag/rules",
			body: CreateFlagRuleRequest{
				Priority: 1,
				Rollout:  50,
				Variant:  json.RawMessage(`{"enabled": true}`),
			},
			handler: handler.CreateFlagRule,
		},
		{
			name:   "UpdateFlag without auth",
			method: "PATCH",
			path:   "/v1/flags/test_flag",
			body: UpdateFlagRequest{
				Enabled: boolPtr(true),
			},
			handler: handler.UpdateFlag,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			reqBody, err := json.Marshal(tt.body)
			if err != nil {
				t.Fatalf("Failed to marshal request body: %v", err)
			}

			// Create request without auth context
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			// Add mux vars if needed
			if tt.method == "POST" && tt.path == "/v1/flags/test_flag/rules" {
				req = mux.SetURLVars(req, map[string]string{"key": "test_flag"})
			} else if tt.method == "PATCH" {
				req = mux.SetURLVars(req, map[string]string{"key": "test_flag"})
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			tt.handler(rr, req)

			// Should return authentication error
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, rr.Code)
			}

			var errorResp ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&errorResp); err != nil {
				t.Errorf("Failed to decode error response: %v", err)
			}

			if errorResp.Error.Code != "AUTHENTICATION_REQUIRED" {
				t.Errorf("Expected error code 'AUTHENTICATION_REQUIRED', got %s", errorResp.Error.Code)
			}
		})
	}
}

// Helper functions for creating pointers
func boolPtr(b bool) *bool {
	return &b
}