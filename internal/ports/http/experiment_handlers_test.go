package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/latoulicious/RoutePilot/internal/core/flags"
	"github.com/latoulicious/RoutePilot/internal/ports/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockExperimentRepository is a mock implementation of ExperimentRepository
type MockExperimentRepository struct {
	mock.Mock
}

func (m *MockExperimentRepository) GetExperimentByFlagID(ctx context.Context, flagID uuid.UUID) (*flags.Experiment, error) {
	args := m.Called(ctx, flagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*flags.Experiment), args.Error(1)
}

func (m *MockExperimentRepository) GetExperimentByTenantKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Experiment, error) {
	args := m.Called(ctx, tenantID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*flags.Experiment), args.Error(1)
}

func (m *MockExperimentRepository) GetExperimentVariants(ctx context.Context, experimentID uuid.UUID) ([]*flags.ExperimentVariant, error) {
	args := m.Called(ctx, experimentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*flags.ExperimentVariant), args.Error(1)
}

func (m *MockExperimentRepository) CreateExperiment(ctx context.Context, experiment *flags.Experiment) error {
	args := m.Called(ctx, experiment)
	return args.Error(0)
}

func (m *MockExperimentRepository) UpdateExperiment(ctx context.Context, experimentID uuid.UUID, status flags.ExperimentStatus) error {
	args := m.Called(ctx, experimentID, status)
	return args.Error(0)
}

func (m *MockExperimentRepository) CreateExperimentVariant(ctx context.Context, variant *flags.ExperimentVariant) error {
	args := m.Called(ctx, variant)
	return args.Error(0)
}

// MockFlagRepository is a mock implementation of FlagRepository for experiment tests
type MockFlagRepositoryForExperiments struct {
	mock.Mock
}

func (m *MockFlagRepositoryForExperiments) GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*flags.Flag, error) {
	args := m.Called(ctx, tenantID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*flags.Flag), args.Error(1)
}

func (m *MockFlagRepositoryForExperiments) GetFlagByID(ctx context.Context, flagID uuid.UUID) (*flags.Flag, error) {
	args := m.Called(ctx, flagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*flags.Flag), args.Error(1)
}

func (m *MockFlagRepositoryForExperiments) GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*flags.FlagRule, error) {
	args := m.Called(ctx, flagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*flags.FlagRule), args.Error(1)
}

func (m *MockFlagRepositoryForExperiments) CreateFlag(ctx context.Context, flag *flags.Flag) error {
	args := m.Called(ctx, flag)
	return args.Error(0)
}

func (m *MockFlagRepositoryForExperiments) UpdateFlag(ctx context.Context, flagID uuid.UUID, updates flags.FlagUpdates) error {
	args := m.Called(ctx, flagID, updates)
	return args.Error(0)
}

func (m *MockFlagRepositoryForExperiments) DeleteFlag(ctx context.Context, flagID uuid.UUID) error {
	args := m.Called(ctx, flagID)
	return args.Error(0)
}

// MockCacheInvalidator is a mock implementation of CacheInvalidator
type MockCacheInvalidator struct {
	mock.Mock
}

func (m *MockCacheInvalidator) InvalidateFlag(tenantID uuid.UUID, flagKey string) {
	m.Called(tenantID, flagKey)
}

func TestExperimentHandler_CreateExperiment_Success(t *testing.T) {
	// Setup
	mockExperimentRepo := new(MockExperimentRepository)
	mockFlagRepo := new(MockFlagRepositoryForExperiments)
	mockCache := new(MockCacheInvalidator)
	
	handler := NewExperimentHandler(mockExperimentRepo, mockFlagRepo, mockCache)
	
	tenantID := uuid.New()
	flagID := uuid.New()
	
	// Mock flag exists
	flag := &flags.Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test_flag",
		Type:     flags.FlagTypeBoolean,
		Enabled:  true,
	}
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test_flag").Return(flag, nil)
	
	// Mock experiment creation
	mockExperimentRepo.On("CreateExperiment", mock.Anything, mock.MatchedBy(func(exp *flags.Experiment) bool {
		return exp.Key == "test_experiment" && exp.FlagID == flagID && exp.Traffic == 50
	})).Return(nil)
	
	// Create request
	reqBody := CreateExperimentRequest{
		Key:     "test_experiment",
		FlagKey: "test_flag",
		Traffic: 50,
	}
	reqJSON, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("POST", "/v1/experiments", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)
	
	rr := httptest.NewRecorder()
	
	// Execute
	handler.CreateExperiment(rr, req)
	
	// Assert
	assert.Equal(t, http.StatusCreated, rr.Code)
	
	var response CreateExperimentResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test_experiment", response.Key)
	assert.Equal(t, "test_flag", response.FlagKey)
	assert.Equal(t, flags.ExperimentStatusDraft, response.Status)
	assert.Equal(t, 50, response.Traffic)
	assert.Empty(t, response.Variants)
	
	mockExperimentRepo.AssertExpectations(t)
	mockFlagRepo.AssertExpectations(t)
}

func TestExperimentHandler_CreateExperiment_FlagNotFound(t *testing.T) {
	// Setup
	mockExperimentRepo := new(MockExperimentRepository)
	mockFlagRepo := new(MockFlagRepositoryForExperiments)
	mockCache := new(MockCacheInvalidator)
	
	handler := NewExperimentHandler(mockExperimentRepo, mockFlagRepo, mockCache)
	
	tenantID := uuid.New()
	
	// Mock flag not found
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "nonexistent_flag").Return(nil, fmt.Errorf("flag not found"))
	
	// Create request
	reqBody := CreateExperimentRequest{
		Key:     "test_experiment",
		FlagKey: "nonexistent_flag",
		Traffic: 50,
	}
	reqJSON, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("POST", "/v1/experiments", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)
	
	rr := httptest.NewRecorder()
	
	// Execute
	handler.CreateExperiment(rr, req)
	
	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "FLAG_NOT_FOUND", response.Error.Code)
	
	mockFlagRepo.AssertExpectations(t)
}

func TestExperimentHandler_CreateVariant_Success(t *testing.T) {
	// Setup
	mockExperimentRepo := new(MockExperimentRepository)
	mockFlagRepo := new(MockFlagRepositoryForExperiments)
	mockCache := new(MockCacheInvalidator)
	
	handler := NewExperimentHandler(mockExperimentRepo, mockFlagRepo, mockCache)
	
	tenantID := uuid.New()
	experimentID := uuid.New()
	
	// Mock experiment exists
	experiment := &flags.Experiment{
		ID:       experimentID,
		TenantID: tenantID,
		Key:      "test_experiment",
		Status:   flags.ExperimentStatusDraft,
		Traffic:  50,
		Variants: []*flags.ExperimentVariant{},
	}
	mockExperimentRepo.On("GetExperimentByTenantKey", mock.Anything, tenantID, "test_experiment").Return(experiment, nil)
	
	// Mock variant creation
	mockExperimentRepo.On("CreateExperimentVariant", mock.Anything, mock.MatchedBy(func(variant *flags.ExperimentVariant) bool {
		return variant.Name == "control" && variant.Weight == 50 && variant.ExperimentID == experimentID
	})).Return(nil)
	
	// Create request
	variantValue := json.RawMessage(`{"enabled":false}`)
	reqBody := CreateVariantRequest{
		Name:   "control",
		Weight: 50,
		Value:  variantValue,
	}
	reqJSON, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("POST", "/v1/experiments/test_experiment/variants", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)
	
	// Add URL vars
	req = mux.SetURLVars(req, map[string]string{"key": "test_experiment"})
	
	rr := httptest.NewRecorder()
	
	// Execute
	handler.CreateVariant(rr, req)
	
	// Assert
	assert.Equal(t, http.StatusCreated, rr.Code)
	
	var response CreateVariantResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "control", response.Name)
	assert.Equal(t, 50, response.Weight)
	assert.Equal(t, experimentID, response.ExperimentID)
	assert.Equal(t, variantValue, response.Value)
	
	mockExperimentRepo.AssertExpectations(t)
}

func TestExperimentHandler_UpdateExperiment_Success(t *testing.T) {
	// Setup
	mockExperimentRepo := new(MockExperimentRepository)
	mockFlagRepo := new(MockFlagRepositoryForExperiments)
	mockCache := new(MockCacheInvalidator)
	
	handler := NewExperimentHandler(mockExperimentRepo, mockFlagRepo, mockCache)
	
	tenantID := uuid.New()
	experimentID := uuid.New()
	flagID := uuid.New()
	
	// Mock experiment exists
	experiment := &flags.Experiment{
		ID:       experimentID,
		TenantID: tenantID,
		Key:      "test_experiment",
		FlagID:   flagID,
		Status:   flags.ExperimentStatusDraft,
		Traffic:  50,
		Variants: []*flags.ExperimentVariant{},
	}
	mockExperimentRepo.On("GetExperimentByTenantKey", mock.Anything, tenantID, "test_experiment").Return(experiment, nil)
	
	// Mock experiment update
	newStatus := flags.ExperimentStatusRunning
	mockExperimentRepo.On("UpdateExperiment", mock.Anything, experimentID, newStatus).Return(nil)
	
	// Mock flag for cache invalidation
	flag := &flags.Flag{
		ID:  flagID,
		Key: "test_flag",
	}
	mockFlagRepo.On("GetFlagByID", mock.Anything, flagID).Return(flag, nil)
	mockCache.On("InvalidateFlag", tenantID, "test_flag").Return()
	
	// Create request
	reqBody := UpdateExperimentRequest{
		Status: &newStatus,
	}
	reqJSON, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("PATCH", "/v1/experiments/test_experiment", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)
	
	// Add URL vars
	req = mux.SetURLVars(req, map[string]string{"key": "test_experiment"})
	
	rr := httptest.NewRecorder()
	
	// Execute
	handler.UpdateExperiment(rr, req)
	
	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)
	
	var response UpdateExperimentResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test_experiment", response.Key)
	assert.Equal(t, flags.ExperimentStatusRunning, response.Status)
	assert.Equal(t, 50, response.Traffic)
	
	mockExperimentRepo.AssertExpectations(t)
	mockFlagRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestExperimentHandler_CreateExperiment_InvalidTraffic(t *testing.T) {
	// Setup
	mockExperimentRepo := new(MockExperimentRepository)
	mockFlagRepo := new(MockFlagRepositoryForExperiments)
	mockCache := new(MockCacheInvalidator)
	
	handler := NewExperimentHandler(mockExperimentRepo, mockFlagRepo, mockCache)
	
	tenantID := uuid.New()
	
	// Create request with invalid traffic
	reqBody := CreateExperimentRequest{
		Key:     "test_experiment",
		FlagKey: "test_flag",
		Traffic: 150, // Invalid: > 100
	}
	reqJSON, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("POST", "/v1/experiments", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)
	
	rr := httptest.NewRecorder()
	
	// Execute
	handler.CreateExperiment(rr, req)
	
	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_TRAFFIC", response.Error.Code)
}

func TestExperimentHandler_CreateVariant_InvalidWeight(t *testing.T) {
	// Setup
	mockExperimentRepo := new(MockExperimentRepository)
	mockFlagRepo := new(MockFlagRepositoryForExperiments)
	mockCache := new(MockCacheInvalidator)
	
	handler := NewExperimentHandler(mockExperimentRepo, mockFlagRepo, mockCache)
	
	tenantID := uuid.New()
	
	// Create request with invalid weight
	variantValue := json.RawMessage(`{"enabled":false}`)
	reqBody := CreateVariantRequest{
		Name:   "control",
		Weight: 150, // Invalid: > 100
		Value:  variantValue,
	}
	reqJSON, _ := json.Marshal(reqBody)
	
	req := httptest.NewRequest("POST", "/v1/experiments/test_experiment/variants", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	
	// Add auth context
	authCtx := &middleware.AuthContext{
		TenantID: tenantID,
		APIKeyID: uuid.New(),
	}
	ctx := context.WithValue(req.Context(), "auth", authCtx)
	req = req.WithContext(ctx)
	
	// Add URL vars
	req = mux.SetURLVars(req, map[string]string{"key": "test_experiment"})
	
	rr := httptest.NewRecorder()
	
	// Execute
	handler.CreateVariant(rr, req)
	
	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	
	var response ErrorResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "INVALID_WEIGHT", response.Error.Code)
}