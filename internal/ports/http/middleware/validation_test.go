package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationMiddleware_ValidRequest(t *testing.T) {
	middleware := NewValidationMiddleware()

	req := httptest.NewRequest("GET", "/v1/flags/test_flag/eval", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-key")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestValidationMiddleware_RequestTooLarge(t *testing.T) {
	middleware := NewValidationMiddleware()

	// Create request with content length exceeding limit
	req := httptest.NewRequest("POST", "/test", nil)
	req.ContentLength = MaxRequestSize + 1

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Contains(t, rr.Body.String(), "REQUEST_TOO_LARGE")
}

func TestValidationMiddleware_InvalidPath(t *testing.T) {
	middleware := NewValidationMiddleware()

	tests := []struct {
		name string
		path string
	}{
		{"path traversal", "/v1/../admin"},
		{"double slash", "/v1//flags"},
		{"null byte", "/v1/flags%00"},
		{"invalid characters", "/v1/flags/<script>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)

			rr := httptest.NewRecorder()
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called")
			})

			handler := middleware.Middleware(testHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), "INVALID_PATH")
		})
	}
}

func TestValidationMiddleware_InvalidHeaders(t *testing.T) {
	middleware := NewValidationMiddleware()

	tests := []struct {
		name   string
		header string
		value  string
	}{
		{"control character in value", "X-Test", "test\x01value"},
		{"header too large", "X-Test", strings.Repeat("a", MaxHeaderSize+1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(tt.header, tt.value)

			rr := httptest.NewRecorder()
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called")
			})

			handler := middleware.Middleware(testHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), "INVALID_HEADERS")
		})
	}
}

func TestValidationMiddleware_ValidJSON(t *testing.T) {
	middleware := NewValidationMiddleware()

	jsonBody := `{"flag_key": "test_flag", "subject_id": "user123"}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify body hash was added
		assert.NotEmpty(t, r.Header.Get("X-Body-Hash"))
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestValidationMiddleware_InvalidJSON(t *testing.T) {
	middleware := NewValidationMiddleware()

	tests := []struct {
		name string
		body string
	}{
		{"malformed JSON", `{"invalid": json}`},
		{"potential injection", `{"__proto__": "malicious"}`},
		{"constructor injection", `{"constructor": {"prototype": {}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called")
			})

			handler := middleware.Middleware(testHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Contains(t, rr.Body.String(), "INVALID_REQUEST_BODY")
		})
	}
}

func TestValidateFlagKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"valid key", "test_flag_123", true},
		{"valid simple key", "flag", true},
		{"empty key", "", false},
		{"too long key", strings.Repeat("a", 101), false},
		{"invalid characters", "test-flag", false},
		{"invalid characters space", "test flag", false},
		{"invalid characters special", "test@flag", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateFlagKey(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateUUID(t *testing.T) {
	tests := []struct {
		name     string
		uuid     string
		expected bool
	}{
		{"valid UUID", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid UUID lowercase", "550e8400-e29b-41d4-a716-446655440000", true},
		{"valid UUID uppercase", "550E8400-E29B-41D4-A716-446655440000", true},
		{"invalid UUID", "invalid-uuid", false},
		{"empty string", "", false},
		{"too short", "550e8400-e29b", false},
		{"wrong format", "550e8400e29b41d4a716446655440000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateUUID(tt.uuid)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal string", "hello world", "hello world"},
		{"string with tabs", "hello\tworld", "hello\tworld"},
		{"string with newlines", "hello\nworld", "hello\nworld"},
		{"string with control chars", "hello\x01world", "helloworld"},
		{"string with leading/trailing spaces", "  hello world  ", "hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}