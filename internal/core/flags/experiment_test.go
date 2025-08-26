package flags

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing - using existing mocks from evaluator_test.go



func TestExperimentEngine_ProcessExperiment_ExperimentNotRunning(t *testing.T) {
	mockExperimentRepo := &MockExperimentRepository{}
	mockAssignmentRepo := &MockAssignmentRepository{}
	
	engine := NewExperimentEngine(mockExperimentRepo, mockAssignmentRepo)
	
	flag := &Flag{
		ID:   uuid.New(),
		Salt: "test-salt",
	}
	
	assignment := &Assignment{
		SubjectID:     "user123",
		Bucket:        1000,
		ChosenVariant: json.RawMessage(`{"enabled": true}`),
	}
	
	// Test with paused experiment
	experiment := &Experiment{
		ID:      uuid.New(),
		Status:  ExperimentStatusPaused,
		Traffic: 100,
	}
	
	result, err := engine.ProcessExperiment(context.Background(), flag, assignment, experiment)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, assignment.ChosenVariant, result.Value)
	assert.Nil(t, result.AssignedVariant)
}

func TestExperimentEngine_ProcessExperiment_SubjectNotInTraffic(t *testing.T) {
	mockExperimentRepo := &MockExperimentRepository{}
	mockAssignmentRepo := &MockAssignmentRepository{}
	
	engine := NewExperimentEngine(mockExperimentRepo, mockAssignmentRepo)
	
	flag := &Flag{
		ID:   uuid.New(),
		Salt: "test-salt",
	}
	
	assignment := &Assignment{
		SubjectID:     "user123",
		Bucket:        1000,
		ChosenVariant: json.RawMessage(`{"enabled": true}`),
	}
	
	// Test with low traffic experiment (subject won't qualify)
	experiment := &Experiment{
		ID:      uuid.New(),
		Status:  ExperimentStatusRunning,
		Traffic: 1, // Very low traffic percentage
	}
	
	result, err := engine.ProcessExperiment(context.Background(), flag, assignment, experiment)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, assignment.ChosenVariant, result.Value)
	assert.Nil(t, result.AssignedVariant)
}

func TestExperimentEngine_ProcessExperiment_BasicVariantAssignment(t *testing.T) {
	mockExperimentRepo := &MockExperimentRepository{}
	mockAssignmentRepo := &MockAssignmentRepository{}
	
	engine := NewExperimentEngine(mockExperimentRepo, mockAssignmentRepo)
	
	flag := &Flag{
		ID:   uuid.New(),
		Salt: "test-salt",
	}
	
	assignment := &Assignment{
		FlagID:        flag.ID,
		SubjectID:     "user123",
		Bucket:        1000,
		ChosenVariant: json.RawMessage(`{"enabled": true}`),
	}
	
	experiment := &Experiment{
		ID:      uuid.New(),
		Status:  ExperimentStatusRunning,
		Traffic: 100, // Full traffic
	}
	
	variants := []*ExperimentVariant{
		{
			ID:     uuid.New(),
			Name:   "control",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": false}`),
		},
		{
			ID:     uuid.New(),
			Name:   "treatment",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": true, "feature": "new"}`),
		},
	}
	
	// Mock repository calls
	mockExperimentRepo.On("GetExperimentVariants", mock.Anything, experiment.ID).Return(variants, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flag.ID, assignment.SubjectID).Return(nil, assert.AnError)
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)
	
	result, err := engine.ProcessExperiment(context.Background(), flag, assignment, experiment)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.AssignedVariant)
	assert.Contains(t, []string{"control", "treatment"}, result.AssignedVariant.Name)
	
	mockExperimentRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestExperimentEngine_ProcessExperiment_StickyAssignment(t *testing.T) {
	mockExperimentRepo := &MockExperimentRepository{}
	mockAssignmentRepo := &MockAssignmentRepository{}
	
	engine := NewExperimentEngine(mockExperimentRepo, mockAssignmentRepo)
	
	flag := &Flag{
		ID:   uuid.New(),
		Salt: "test-salt",
	}
	
	assignment := &Assignment{
		FlagID:        flag.ID,
		SubjectID:     "user123",
		Bucket:        1000,
		ChosenVariant: json.RawMessage(`{"enabled": true}`),
	}
	
	experiment := &Experiment{
		ID:      uuid.New(),
		Status:  ExperimentStatusRunning,
		Traffic: 100,
	}
	
	variants := []*ExperimentVariant{
		{
			ID:     uuid.New(),
			Name:   "control",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": false}`),
		},
		{
			ID:     uuid.New(),
			Name:   "treatment",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": true, "feature": "new"}`),
		},
	}
	
	// Existing assignment matches treatment variant
	existingAssignment := &Assignment{
		FlagID:        flag.ID,
		SubjectID:     "user123",
		Bucket:        1000,
		ChosenVariant: json.RawMessage(`{"enabled": true, "feature": "new"}`),
		AssignedAt:    time.Now().Add(-time.Hour),
	}
	
	// Mock repository calls
	mockExperimentRepo.On("GetExperimentVariants", mock.Anything, experiment.ID).Return(variants, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flag.ID, assignment.SubjectID).Return(existingAssignment, nil)
	
	result, err := engine.ProcessExperiment(context.Background(), flag, assignment, experiment)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.AssignedVariant)
	assert.Equal(t, "treatment", result.AssignedVariant.Name)
	assert.Equal(t, variants[1].Value, result.Value)
	
	mockExperimentRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestExperimentEngine_ProcessExperiment_NoVariants(t *testing.T) {
	mockExperimentRepo := &MockExperimentRepository{}
	mockAssignmentRepo := &MockAssignmentRepository{}
	
	engine := NewExperimentEngine(mockExperimentRepo, mockAssignmentRepo)
	
	flag := &Flag{
		ID:   uuid.New(),
		Salt: "test-salt",
	}
	
	assignment := &Assignment{
		SubjectID:     "user123",
		Bucket:        1000,
		ChosenVariant: json.RawMessage(`{"enabled": true}`),
	}
	
	experiment := &Experiment{
		ID:      uuid.New(),
		Status:  ExperimentStatusRunning,
		Traffic: 100,
	}
	
	// Mock repository calls - no variants
	mockExperimentRepo.On("GetExperimentVariants", mock.Anything, experiment.ID).Return([]*ExperimentVariant{}, nil)
	
	result, err := engine.ProcessExperiment(context.Background(), flag, assignment, experiment)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, assignment.ChosenVariant, result.Value)
	assert.Nil(t, result.AssignedVariant)
	
	mockExperimentRepo.AssertExpectations(t)
}

func TestExperimentEngine_isExperimentActive(t *testing.T) {
	engine := &ExperimentEngineImpl{}
	
	tests := []struct {
		name     string
		status   ExperimentStatus
		expected bool
	}{
		{"Running experiment", ExperimentStatusRunning, true},
		{"Draft experiment", ExperimentStatusDraft, false},
		{"Paused experiment", ExperimentStatusPaused, false},
		{"Stopped experiment", ExperimentStatusStopped, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			experiment := &Experiment{Status: tt.status}
			result := engine.isExperimentActive(experiment)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExperimentEngine_isSubjectInTraffic(t *testing.T) {
	engine := &ExperimentEngineImpl{}
	
	tests := []struct {
		name               string
		salt               string
		subjectID          string
		trafficPercentage  int
		expectedInTraffic  bool
	}{
		{"Zero traffic", "salt", "user123", 0, false},
		{"Full traffic", "salt", "user123", 100, true},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.isSubjectInTraffic(tt.salt, tt.subjectID, tt.trafficPercentage)
			assert.Equal(t, tt.expectedInTraffic, result)
		})
	}
	
	// Test deterministic behavior - same inputs should always give same result
	t.Run("Deterministic behavior", func(t *testing.T) {
		result1 := engine.isSubjectInTraffic("salt", "user123", 50)
		result2 := engine.isSubjectInTraffic("salt", "user123", 50)
		assert.Equal(t, result1, result2, "Same inputs should give same result")
		
		// Different users should potentially give different results
		result3 := engine.isSubjectInTraffic("salt", "user456", 50)
		// We don't assert the specific value, just that it's deterministic
		result4 := engine.isSubjectInTraffic("salt", "user456", 50)
		assert.Equal(t, result3, result4, "Same inputs should give same result")
	})
}

func TestExperimentEngine_selectVariantByWeight(t *testing.T) {
	engine := &ExperimentEngineImpl{}
	
	variants := []*ExperimentVariant{
		{
			ID:     uuid.New(),
			Name:   "control",
			Weight: 30,
			Value:  json.RawMessage(`{"enabled": false}`),
		},
		{
			ID:     uuid.New(),
			Name:   "treatment",
			Weight: 70,
			Value:  json.RawMessage(`{"enabled": true}`),
		},
	}
	
	// Test deterministic selection
	result1 := engine.selectVariantByWeight("salt", "user123", variants)
	result2 := engine.selectVariantByWeight("salt", "user123", variants)
	
	assert.NotNil(t, result1)
	assert.NotNil(t, result2)
	assert.Equal(t, result1.Name, result2.Name) // Should be deterministic
	
	// Test with empty variants
	emptyResult := engine.selectVariantByWeight("salt", "user123", []*ExperimentVariant{})
	assert.Nil(t, emptyResult)
	
	// Test with zero weights
	zeroWeightVariants := []*ExperimentVariant{
		{Name: "control", Weight: 0},
		{Name: "treatment", Weight: 0},
	}
	zeroResult := engine.selectVariantByWeight("salt", "user123", zeroWeightVariants)
	assert.Nil(t, zeroResult)
}

func TestExperimentEngine_WeightDistribution(t *testing.T) {
	engine := &ExperimentEngineImpl{}
	
	variants := []*ExperimentVariant{
		{
			ID:     uuid.New(),
			Name:   "control",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": false}`),
		},
		{
			ID:     uuid.New(),
			Name:   "treatment",
			Weight: 50,
			Value:  json.RawMessage(`{"enabled": true}`),
		},
	}
	
	// Test distribution with multiple subjects
	controlCount := 0
	treatmentCount := 0
	totalTests := 1000
	
	for i := 0; i < totalTests; i++ {
		subjectID := fmt.Sprintf("user%d", i)
		result := engine.selectVariantByWeight("salt", subjectID, variants)
		
		if result != nil {
			if result.Name == "control" {
				controlCount++
			} else if result.Name == "treatment" {
				treatmentCount++
			}
		}
	}
	
	// With 50/50 weights, we expect roughly equal distribution
	// Allow for some variance (40-60% range)
	controlPercentage := float64(controlCount) / float64(totalTests) * 100
	treatmentPercentage := float64(treatmentCount) / float64(totalTests) * 100
	
	assert.True(t, controlPercentage >= 40 && controlPercentage <= 60, 
		"Control percentage should be between 40-60%%, got %.2f%%", controlPercentage)
	assert.True(t, treatmentPercentage >= 40 && treatmentPercentage <= 60, 
		"Treatment percentage should be between 40-60%%, got %.2f%%", treatmentPercentage)
}