package flags

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFlagEvaluator_EvaluateFlag_WithExperiment(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	experimentID := uuid.New()
	
	// Create enabled flag
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "experiment-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeJSON,
	}

	// Create rule
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  100,
			Variant:  json.RawMessage(`{"enabled": true, "version": "control"}`),
		},
	}

	// Create experiment
	experiment := &Experiment{
		ID:      experimentID,
		Key:     "test-experiment",
		FlagID:  flagID,
		Status:  ExperimentStatusRunning,
		Traffic: 100,
	}

	// Assignment will be created during evaluation

	// Expected experiment result
	experimentResult := &ExperimentResult{
		AssignedVariant: &ExperimentVariant{
			ID:     uuid.New(),
			Name:   "treatment",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": true, "version": "treatment"}`),
		},
		Value: json.RawMessage(`{"enabled": true, "version": "treatment"}`),
	}

	// Mock repository calls
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "experiment-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)
	
	// Experiment-related mocks
	mockExperimentRepo.On("GetExperimentByFlagID", mock.Anything, flagID).Return(experiment, nil)
	mockExperimentEngine.On("ProcessExperiment", mock.Anything, flag, mock.AnythingOfType("*flags.Assignment"), experiment).Return(experimentResult, nil)

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "experiment-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "experiment-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`{"enabled": true, "version": "treatment"}`), result.Value)
	assert.NotNil(t, result.ExperimentKey)
	assert.Equal(t, "test-experiment", *result.ExperimentKey)
	assert.NotNil(t, result.VariantName)
	assert.Equal(t, "treatment", *result.VariantName)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
	mockExperimentRepo.AssertExpectations(t)
	mockExperimentEngine.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_NoExperiment(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	// Create enabled flag
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "normal-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	// Create rule
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  100,
			Variant:  json.RawMessage(`true`),
		},
	}

	// Mock repository calls
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "normal-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)
	
	// No experiment for this flag
	mockExperimentRepo.On("GetExperimentByFlagID", mock.Anything, flagID).Return(nil, errors.New("no experiment"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "normal-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "normal-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`true`), result.Value)
	assert.Nil(t, result.ExperimentKey)
	assert.Nil(t, result.VariantName)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
	mockExperimentRepo.AssertExpectations(t)
	// Experiment engine should not be called when no experiment exists
	mockExperimentEngine.AssertNotCalled(t, "ProcessExperiment")
}

func TestFlagEvaluator_EvaluateFlag_ExperimentProcessingFailure(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	experimentID := uuid.New()
	
	// Create enabled flag
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "experiment-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	// Create rule
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  100,
			Variant:  json.RawMessage(`true`),
		},
	}

	// Create experiment
	experiment := &Experiment{
		ID:      experimentID,
		Key:     "test-experiment",
		FlagID:  flagID,
		Status:  ExperimentStatusRunning,
		Traffic: 100,
	}

	// Mock repository calls
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "experiment-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)
	
	// Experiment exists but processing fails
	mockExperimentRepo.On("GetExperimentByFlagID", mock.Anything, flagID).Return(experiment, nil)
	mockExperimentEngine.On("ProcessExperiment", mock.Anything, flag, mock.AnythingOfType("*flags.Assignment"), experiment).Return(nil, errors.New("experiment processing failed"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "experiment-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert - should fall back to normal flag evaluation
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "experiment-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`true`), result.Value)
	assert.Nil(t, result.ExperimentKey)
	assert.Nil(t, result.VariantName)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
	mockExperimentRepo.AssertExpectations(t)
	mockExperimentEngine.AssertExpectations(t)
}