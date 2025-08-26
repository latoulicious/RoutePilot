package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockIdempotencyRepository is a mock implementation of IdempotencyRepository
type MockIdempotencyRepository struct {
	mock.Mock
}

func (m *MockIdempotencyRepository) GetIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID) (*IdempotencyKey, error) {
	args := m.Called(ctx, tenantID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*IdempotencyKey), args.Error(1)
}

func (m *MockIdempotencyRepository) CreateIdempotencyKey(ctx context.Context, tenantID, key uuid.UUID, method string, pathHash []byte, status int32) (*IdempotencyKey, error) {
	args := m.Called(ctx, tenantID, key, method, pathHash, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*IdempotencyKey), args.Error(1)
}

func TestIdempotencyMiddleware_GetRequest(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	// GET requests should pass through without idempotency processing
	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertNotCalled(t, "GetIdempotencyKey")
	mockRepo.AssertNotCalled(t, "CreateIdempotencyKey")
}

func TestIdempotencyMiddleware_NoIdempotencyKey(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	req := httptest.NewRequest("POST", "/test", nil)
	rr := httptest.NewRecorder()

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	// POST requests without idempotency key should pass through
	assert.Equal(t, http.StatusOK, rr.Code)
	mockRepo.AssertNotCalled(t, "GetIdempotencyKey")
	mockRepo.AssertNotCalled(t, "CreateIdempotencyKey")
}

func TestIdempotencyMiddleware_InvalidIdempotencyKey(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", "invalid-uuid")

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_IDEMPOTENCY_KEY")
}

func TestIdempotencyMiddleware_NoAuthContext(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", uuid.New().String())

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "AUTHENTICATION_REQUIRED")
}

func TestIdempotencyMiddleware_ExistingKeyMatchingRequest(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	tenantID := uuid.New()
	idempotencyKey := uuid.New()

	// Setup auth context
	authCtx := &AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}

	// Mock existing idempotency key
	existingKey := &IdempotencyKey{
		TenantID:  tenantID,
		Key:       idempotencyKey,
		Method:    "POST",
		PathHash:  middleware.createPathHash("POST", "/test", ""),
		Status:    200,
		CreatedAt: time.Now(),
	}

	mockRepo.On("GetIdempotencyKey", mock.Anything, tenantID, idempotencyKey).Return(existingKey, nil)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", idempotencyKey.String())
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code) // Status from existing key
	assert.Contains(t, rr.Body.String(), "Request already processed")
	mockRepo.AssertExpectations(t)
}

func TestIdempotencyMiddleware_ExistingKeyDifferentRequest(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	tenantID := uuid.New()
	idempotencyKey := uuid.New()

	// Setup auth context
	authCtx := &AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}

	// Mock existing idempotency key with different path hash
	existingKey := &IdempotencyKey{
		TenantID:  tenantID,
		Key:       idempotencyKey,
		Method:    "POST",
		PathHash:  []byte("different-hash"),
		Status:    200,
		CreatedAt: time.Now(),
	}

	mockRepo.On("GetIdempotencyKey", mock.Anything, tenantID, idempotencyKey).Return(existingKey, nil)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", idempotencyKey.String())
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "IDEMPOTENCY_KEY_CONFLICT")
	mockRepo.AssertExpectations(t)
}

func TestIdempotencyMiddleware_NewIdempotencyKey(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	tenantID := uuid.New()
	idempotencyKey := uuid.New()

	// Setup auth context
	authCtx := &AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}

	// Mock idempotency key not found, then successful creation
	mockRepo.On("GetIdempotencyKey", mock.Anything, tenantID, idempotencyKey).Return(nil, fmt.Errorf("not found"))
	
	newKey := &IdempotencyKey{
		TenantID:  tenantID,
		Key:       idempotencyKey,
		Method:    "POST",
		Status:    0,
		CreatedAt: time.Now(),
	}
	mockRepo.On("CreateIdempotencyKey", mock.Anything, tenantID, idempotencyKey, "POST", mock.AnythingOfType("[]uint8"), int32(0)).Return(newKey, nil)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", idempotencyKey.String())
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify idempotency key is in context
		key, ok := GetIdempotencyKey(r)
		assert.True(t, ok)
		assert.Equal(t, idempotencyKey, key)
		w.WriteHeader(http.StatusCreated)
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	mockRepo.AssertExpectations(t)
}

func TestIdempotencyMiddleware_CreateKeyConflict(t *testing.T) {
	mockRepo := new(MockIdempotencyRepository)
	middleware := NewIdempotencyMiddleware(mockRepo)

	tenantID := uuid.New()
	idempotencyKey := uuid.New()

	// Setup auth context
	authCtx := &AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}

	// Mock idempotency key not found, then creation conflict
	mockRepo.On("GetIdempotencyKey", mock.Anything, tenantID, idempotencyKey).Return(nil, fmt.Errorf("not found"))
	mockRepo.On("CreateIdempotencyKey", mock.Anything, tenantID, idempotencyKey, "POST", mock.AnythingOfType("[]uint8"), int32(0)).Return(nil, fmt.Errorf("conflict"))

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", idempotencyKey.String())
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	})

	handler := middleware.Middleware(testHandler)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "IDEMPOTENCY_KEY_PROCESSING")
	mockRepo.AssertExpectations(t)
}

func TestGetIdempotencyKey(t *testing.T) {
	// Test with valid idempotency key
	idempotencyKey := uuid.New()

	req := httptest.NewRequest("POST", "/test", nil)
	ctx := context.WithValue(req.Context(), "idempotency_key", idempotencyKey)
	req = req.WithContext(ctx)

	retrievedKey, ok := GetIdempotencyKey(req)
	assert.True(t, ok)
	assert.Equal(t, idempotencyKey, retrievedKey)

	// Test with no idempotency key
	req2 := httptest.NewRequest("POST", "/test", nil)
	retrievedKey2, ok2 := GetIdempotencyKey(req2)
	assert.False(t, ok2)
	assert.Equal(t, uuid.Nil, retrievedKey2)
}