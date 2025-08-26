package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPIKeyRepository is a mock implementation of APIKeyRepository
type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (*APIKey, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) UpdateAPIKeyLastUsed(ctx context.Context, keyID uuid.UUID) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

// MockSecretDecryptor is a mock implementation of SecretDecryptor
type MockSecretDecryptor struct {
	mock.Mock
}

func (m *MockSecretDecryptor) Decrypt(encrypted []byte) ([]byte, error) {
	args := m.Called(encrypted)
	return args.Get(0).([]byte), args.Error(1)
}

func TestHMACAuthMiddleware_ValidSignature(t *testing.T) {
	// Setup
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	keyID := uuid.New()
	tenantID := uuid.New()
	secret := "test-secret-key"
	timestamp := time.Now().Unix()

	// Mock API key
	apiKey := &APIKey{
		ID:        keyID,
		TenantID:  tenantID,
		Name:      "test-key",
		SecretEnc: []byte("encrypted-secret"),
		Active:    true,
		CreatedAt: time.Now(),
	}

	// Setup mocks
	mockRepo.On("GetAPIKeyByID", mock.Anything, keyID).Return(apiKey, nil)
	mockRepo.On("UpdateAPIKeyLastUsed", mock.Anything, keyID).Return(nil)
	mockDecryptor.On("Decrypt", []byte("encrypted-secret")).Return([]byte(secret), nil)

	// Create request with valid signature
	req := httptest.NewRequest("GET", "/v1/flags/test-flag/eval", nil)
	req.Header.Set("Authorization", "Bearer "+keyID.String())
	req.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))

	// Calculate signature
	payload := fmt.Sprintf("%s%s%s%d", req.Method, req.URL.Path, req.URL.RawQuery, timestamp)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	signature := hex.EncodeToString(h.Sum(nil))
	req.Header.Set("X-Signature", signature)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authCtx, ok := GetAuthContext(r)
		assert.True(t, ok)
		assert.Equal(t, tenantID, authCtx.TenantID)
		assert.Equal(t, keyID, authCtx.APIKeyID)
		w.WriteHeader(http.StatusOK)
	})

	// Execute
	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	// Wait for async UpdateAPIKeyLastUsed call
	time.Sleep(10 * time.Millisecond)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertExpectations(t)
	mockDecryptor.AssertExpectations(t)
}

func TestHMACAuthMiddleware_MissingHeaders(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	tests := []struct {
		name        string
		setupReq    func(*http.Request)
		expectedMsg string
	}{
		{
			name: "missing authorization header",
			setupReq: func(req *http.Request) {
				req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
				req.Header.Set("X-Signature", "test-signature")
			},
			expectedMsg: "missing required authentication headers",
		},
		{
			name: "missing timestamp header",
			setupReq: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+uuid.New().String())
				req.Header.Set("X-Signature", "test-signature")
			},
			expectedMsg: "missing required authentication headers",
		},
		{
			name: "missing signature header",
			setupReq: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+uuid.New().String())
				req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
			},
			expectedMsg: "missing required authentication headers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			tt.setupReq(req)

			rr := httptest.NewRecorder()
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Handler should not be called")
			})

			handler := middleware.Middleware(testHandler)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedMsg)
		})
	}
}

func TestHMACAuthMiddleware_InvalidAuthorizationFormat(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "test-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid authorization header format")
}

func TestHMACAuthMiddleware_InvalidAPIKeyFormat(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-uuid")
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "test-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid API key format")
}

func TestHMACAuthMiddleware_TimestampOutsideWindow(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	// Timestamp 10 minutes ago (outside 5-minute window)
	oldTimestamp := time.Now().Unix() - 600

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+uuid.New().String())
	req.Header.Set("X-Timestamp", strconv.FormatInt(oldTimestamp, 10))
	req.Header.Set("X-Signature", "test-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "timestamp outside valid window")
}

func TestHMACAuthMiddleware_APIKeyNotFound(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	keyID := uuid.New()

	// Mock API key not found
	mockRepo.On("GetAPIKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+keyID.String())
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "test-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "API key not found")
	mockRepo.AssertExpectations(t)
}

func TestHMACAuthMiddleware_InactiveAPIKey(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	keyID := uuid.New()

	// Mock inactive API key
	apiKey := &APIKey{
		ID:        keyID,
		TenantID:  uuid.New(),
		Name:      "test-key",
		SecretEnc: []byte("encrypted-secret"),
		Active:    false, // Inactive
		CreatedAt: time.Now(),
	}

	mockRepo.On("GetAPIKeyByID", mock.Anything, keyID).Return(apiKey, nil)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+keyID.String())
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "test-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "API key is inactive")
	mockRepo.AssertExpectations(t)
}

func TestHMACAuthMiddleware_DecryptionFailure(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	keyID := uuid.New()

	// Mock API key
	apiKey := &APIKey{
		ID:        keyID,
		TenantID:  uuid.New(),
		Name:      "test-key",
		SecretEnc: []byte("encrypted-secret"),
		Active:    true,
		CreatedAt: time.Now(),
	}

	mockRepo.On("GetAPIKeyByID", mock.Anything, keyID).Return(apiKey, nil)
	mockDecryptor.On("Decrypt", []byte("encrypted-secret")).Return([]byte{}, fmt.Errorf("decryption failed"))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+keyID.String())
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "test-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to decrypt API key secret")
	mockRepo.AssertExpectations(t)
	mockDecryptor.AssertExpectations(t)
}

func TestHMACAuthMiddleware_InvalidSignature(t *testing.T) {
	mockRepo := new(MockAPIKeyRepository)
	mockDecryptor := new(MockSecretDecryptor)
	middleware := NewHMACAuthMiddleware(mockRepo, mockDecryptor)

	keyID := uuid.New()
	secret := "test-secret-key"

	// Mock API key
	apiKey := &APIKey{
		ID:        keyID,
		TenantID:  uuid.New(),
		Name:      "test-key",
		SecretEnc: []byte("encrypted-secret"),
		Active:    true,
		CreatedAt: time.Now(),
	}

	mockRepo.On("GetAPIKeyByID", mock.Anything, keyID).Return(apiKey, nil)
	mockDecryptor.On("Decrypt", []byte("encrypted-secret")).Return([]byte(secret), nil)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+keyID.String())
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "invalid-signature")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid signature")
	mockRepo.AssertExpectations(t)
	mockDecryptor.AssertExpectations(t)
}

func TestGetAuthContext(t *testing.T) {
	// Test with valid auth context
	authCtx := &AuthContext{
		TenantID: uuid.New(),
		APIKeyID: uuid.New(),
	}

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	retrievedCtx, ok := GetAuthContext(req)
	assert.True(t, ok)
	assert.Equal(t, authCtx.TenantID, retrievedCtx.TenantID)
	assert.Equal(t, authCtx.APIKeyID, retrievedCtx.APIKeyID)

	// Test with no auth context
	req2 := httptest.NewRequest("GET", "/test", nil)
	retrievedCtx2, ok2 := GetAuthContext(req2)
	assert.False(t, ok2)
	assert.Nil(t, retrievedCtx2)
}