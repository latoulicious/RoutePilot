package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
)

// Mock implementations for testing
type mockEvaluator struct {
	result     *flags.EvaluationResult
	err        error
	evalFunc   func(ctx context.Context, req flags.EvaluationRequest) (*flags.EvaluationResult, error)
}

func (m *mockEvaluator) EvaluateFlag(ctx context.Context, req flags.EvaluationRequest) (*flags.EvaluationResult, error) {
	if m.evalFunc != nil {
		return m.evalFunc(ctx, req)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

type mockOutboxRepo struct {
	events []*flags.OutboxEvent
}

func (m *mockOutboxRepo) AddEvent(ctx context.Context, event *flags.OutboxEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, limit int) ([]*flags.OutboxEvent, error) {
	return nil, nil
}

func (m *mockOutboxRepo) MarkPublished(ctx context.Context, eventIDs []uuid.UUID) error {
	return nil
}

func TestFlagHandler_EvaluateFlag(t *testing.T) {
	tests := []struct {
		name           string
		flagKey        string
		subjectID      string
		evaluatorResult *flags.EvaluationResult
		evaluatorError error
		expectedStatus int
		expectedResponse EvaluateFlagResponse
		expectError    bool
	}{
		{
			name:      "successful flag evaluation - enabled",
			flagKey:   "test_flag",
			subjectID: "user123",
			evaluatorResult: &flags.EvaluationResult{
				FlagKey: "test_flag",
				Enabled: true,
				Value:   json.RawMessage(`true`),
				Bucket:  1234,
			},
			expectedStatus: http.StatusOK,
			expectedResponse: EvaluateFlagResponse{
				FlagKey: "test_flag",
				Enabled: true,
				Value:   json.RawMessage(`true`),
				Bucket:  1234,
			},
			expectError: false,
		},
		{
			name:      "successful flag evaluation - disabled",
			flagKey:   "test_flag",
			subjectID: "user456",
			evaluatorResult: &flags.EvaluationResult{
				FlagKey: "test_flag",
				Enabled: false,
				Value:   json.RawMessage(`false`),
				Bucket:  5678,
			},
			expectedStatus: http.StatusOK,
			expectedResponse: EvaluateFlagResponse{
				FlagKey: "test_flag",
				Enabled: false,
				Value:   json.RawMessage(`false`),
				Bucket:  5678,
			},
			expectError: false,
		},
		{
			name:      "successful flag evaluation with experiment",
			flagKey:   "experiment_flag",
			subjectID: "user789",
			evaluatorResult: &flags.EvaluationResult{
				FlagKey:       "experiment_flag",
				Enabled:       true,
				Value:         json.RawMessage(`{"color": "blue"}`),
				Bucket:        9999,
				ExperimentKey: stringPtr("exp_001"),
				VariantName:   stringPtr("variant_a"),
			},
			expectedStatus: http.StatusOK,
			expectedResponse: EvaluateFlagResponse{
				FlagKey:       "experiment_flag",
				Enabled:       true,
				Value:         json.RawMessage(`{"color": "blue"}`),
				Bucket:        9999,
				ExperimentKey: stringPtr("exp_001"),
				VariantName:   stringPtr("variant_a"),
			},
			expectError: false,
		},
		{
			name:           "missing subject_id",
			flagKey:        "test_flag",
			subjectID:      "",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "invalid flag key",
			flagKey:        "invalid-flag!",
			subjectID:      "user123",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock dependencies
			mockEval := &mockEvaluator{
				result: tt.evaluatorResult,
				err:    tt.evaluatorError,
			}
			mockOutbox := &mockOutboxRepo{}

			// Create handler
			handler := NewFlagHandler(mockEval, mockOutbox)

			// Create request
			req := httptest.NewRequest("GET", "/v1/flags/"+tt.flagKey+"/eval", nil)
			if tt.subjectID != "" {
				q := req.URL.Query()
				q.Add("subject_id", tt.subjectID)
				req.URL.RawQuery = q.Encode()
			}

			// Add auth context to request
			tenantID := uuid.New()
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
			handler.EvaluateFlag(rr, req)

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
				if errorResp.Error.Code == "" {
					t.Error("Expected error code in response")
				}
			} else {
				var response EvaluateFlagResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Compare response
				if response.FlagKey != tt.expectedResponse.FlagKey {
					t.Errorf("Expected flag_key %s, got %s", tt.expectedResponse.FlagKey, response.FlagKey)
				}
				if response.Enabled != tt.expectedResponse.Enabled {
					t.Errorf("Expected enabled %v, got %v", tt.expectedResponse.Enabled, response.Enabled)
				}
				if response.Bucket != tt.expectedResponse.Bucket {
					t.Errorf("Expected bucket %d, got %d", tt.expectedResponse.Bucket, response.Bucket)
				}

				// Check experiment fields
				if tt.expectedResponse.ExperimentKey != nil {
					if response.ExperimentKey == nil || *response.ExperimentKey != *tt.expectedResponse.ExperimentKey {
						t.Errorf("Expected experiment_key %v, got %v", tt.expectedResponse.ExperimentKey, response.ExperimentKey)
					}
				}
				if tt.expectedResponse.VariantName != nil {
					if response.VariantName == nil || *response.VariantName != *tt.expectedResponse.VariantName {
						t.Errorf("Expected variant_name %v, got %v", tt.expectedResponse.VariantName, response.VariantName)
					}
				}

				// Verify outbox event was published (with some delay for goroutine)
				time.Sleep(10 * time.Millisecond)
				if len(mockOutbox.events) != 1 {
					t.Errorf("Expected 1 outbox event, got %d", len(mockOutbox.events))
				} else {
					event := mockOutbox.events[0]
					if event.EventType != "flag_exposure" {
						t.Errorf("Expected event type 'flag_exposure', got %s", event.EventType)
					}
					if event.TenantID != tenantID {
						t.Errorf("Expected tenant ID %s, got %s", tenantID, event.TenantID)
					}
				}
			}
		})
	}
}

func TestFlagHandler_EvaluateFlag_Timeout(t *testing.T) {
	// Create mock evaluator that takes longer than timeout
	mockEval := &mockEvaluator{
		result: &flags.EvaluationResult{
			FlagKey: "test_flag",
			Enabled: true,
			Value:   json.RawMessage(`true`),
			Bucket:  1234,
		},
	}

	// Override the EvaluateFlag method to simulate slow response
	mockEval.evalFunc = func(ctx context.Context, req flags.EvaluationRequest) (*flags.EvaluationResult, error) {
		// Wait longer than the 200ms timeout
		select {
		case <-time.After(300 * time.Millisecond):
			return mockEval.result, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	mockOutbox := &mockOutboxRepo{}
	handler := NewFlagHandler(mockEval, mockOutbox)

	// Create request
	req := httptest.NewRequest("GET", "/v1/flags/test_flag/eval?subject_id=user123", nil)

	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: uuid.New(),
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	// Add mux vars
	req = mux.SetURLVars(req, map[string]string{"key": "test_flag"})

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.EvaluateFlag(rr, req)

	// Should return timeout error
	if rr.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status %d, got %d", http.StatusRequestTimeout, rr.Code)
	}

	var errorResp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&errorResp); err != nil {
		t.Errorf("Failed to decode error response: %v", err)
	}

	if errorResp.Error.Code != "EVALUATION_TIMEOUT" {
		t.Errorf("Expected error code 'EVALUATION_TIMEOUT', got %s", errorResp.Error.Code)
	}
}

func TestFlagHandler_TrackConversion(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectError    bool
		expectedErrorCode string
	}{
		{
			name: "successful conversion tracking",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_001",
				SubjectID:     "user123",
				ConversionKey: "purchase",
				Value:         floatPtr(29.99),
				Properties: map[string]interface{}{
					"product_id": "prod_123",
					"category":   "electronics",
				},
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "successful conversion tracking without optional fields",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_002",
				SubjectID:     "user456",
				ConversionKey: "signup",
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "missing experiment_key",
			requestBody: ConversionRequest{
				SubjectID:     "user123",
				ConversionKey: "purchase",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_EXPERIMENT_KEY",
		},
		{
			name: "missing subject_id",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_001",
				ConversionKey: "purchase",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_SUBJECT_ID",
		},
		{
			name: "missing conversion_key",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_001",
				SubjectID:     "user123",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "MISSING_CONVERSION_KEY",
		},
		{
			name: "invalid experiment_key format",
			requestBody: ConversionRequest{
				ExperimentKey: "invalid-exp!",
				SubjectID:     "user123",
				ConversionKey: "purchase",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_EXPERIMENT_KEY",
		},
		{
			name: "invalid conversion_key format",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_001",
				SubjectID:     "user123",
				ConversionKey: "invalid-conversion!",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_CONVERSION_KEY",
		},
		{
			name: "experiment_key too long",
			requestBody: ConversionRequest{
				ExperimentKey: string(make([]byte, 256)), // 256 characters
				SubjectID:     "user123",
				ConversionKey: "purchase",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_EXPERIMENT_KEY",
		},
		{
			name: "subject_id too long",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_001",
				SubjectID:     string(make([]byte, 256)), // 256 characters
				ConversionKey: "purchase",
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_SUBJECT_ID",
		},
		{
			name: "conversion_key too long",
			requestBody: ConversionRequest{
				ExperimentKey: "exp_001",
				SubjectID:     "user123",
				ConversionKey: string(make([]byte, 256)), // 256 characters
			},
			expectedStatus:    http.StatusBadRequest,
			expectError:       true,
			expectedErrorCode: "INVALID_CONVERSION_KEY",
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
			mockEval := &mockEvaluator{}
			mockOutbox := &mockOutboxRepo{}

			// Create handler
			handler := NewFlagHandler(mockEval, mockOutbox)

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
			req := httptest.NewRequest("POST", "/v1/experiments/conversions", 
				bytes.NewReader(reqBody))
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
			handler.TrackConversion(rr, req)

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
				var response ConversionResponse
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}

				// Check response fields
				if response.ConversionID == "" {
					t.Error("Expected conversion_id in response")
				}
				if response.Status != "accepted" {
					t.Errorf("Expected status 'accepted', got %s", response.Status)
				}

				// Verify UUID format
				if _, err := uuid.Parse(response.ConversionID); err != nil {
					t.Errorf("Expected valid UUID for conversion_id, got %s", response.ConversionID)
				}

				// Verify outbox event was published (with some delay for goroutine)
				time.Sleep(10 * time.Millisecond)
				if len(mockOutbox.events) != 1 {
					t.Errorf("Expected 1 outbox event, got %d", len(mockOutbox.events))
				} else {
					event := mockOutbox.events[0]
					if event.EventType != "experiment_conversion" {
						t.Errorf("Expected event type 'experiment_conversion', got %s", event.EventType)
					}
					if event.TenantID != tenantID {
						t.Errorf("Expected tenant ID %s, got %s", tenantID, event.TenantID)
					}

					// Verify payload structure
					var payload map[string]interface{}
					if err := json.Unmarshal(event.Payload, &payload); err != nil {
						t.Errorf("Failed to unmarshal event payload: %v", err)
					} else {
						// Check required fields in payload
						if payload["conversion_id"] == "" {
							t.Error("Expected conversion_id in event payload")
						}
						if payload["experiment_key"] == "" {
							t.Error("Expected experiment_key in event payload")
						}
						if payload["subject_id"] == "" {
							t.Error("Expected subject_id in event payload")
						}
						if payload["conversion_key"] == "" {
							t.Error("Expected conversion_key in event payload")
						}
						if payload["timestamp"] == "" {
							t.Error("Expected timestamp in event payload")
						}

						// Check optional fields if present in request
						if convReq, ok := tt.requestBody.(ConversionRequest); ok {
							if convReq.Value != nil {
								if payload["value"] == nil {
									t.Error("Expected value in event payload")
								}
							}
							if convReq.Properties != nil && len(convReq.Properties) > 0 {
								if payload["properties"] == nil {
									t.Error("Expected properties in event payload")
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestFlagHandler_TrackConversion_NoAuthContext(t *testing.T) {
	// Create mock dependencies
	mockEval := &mockEvaluator{}
	mockOutbox := &mockOutboxRepo{}

	// Create handler
	handler := NewFlagHandler(mockEval, mockOutbox)

	// Create request body
	reqBody := ConversionRequest{
		ExperimentKey: "exp_001",
		SubjectID:     "user123",
		ConversionKey: "purchase",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// Create request without auth context
	req := httptest.NewRequest("POST", "/v1/experiments/conversions", 
		bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	handler.TrackConversion(rr, req)

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
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Helper function to create float64 pointers
func floatPtr(f float64) *float64 {
	return &f
}