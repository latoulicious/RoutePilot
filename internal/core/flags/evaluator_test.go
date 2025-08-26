package flags

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repositories for testing
type MockFlagRepository struct {
	mock.Mock
}

func (m *MockFlagRepository) GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*Flag, error) {
	args := m.Called(ctx, tenantID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Flag), args.Error(1)
}

func (m *MockFlagRepository) GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*FlagRule, error) {
	args := m.Called(ctx, flagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*FlagRule), args.Error(1)
}

type MockAssignmentRepository struct {
	mock.Mock
}

func (m *MockAssignmentRepository) GetAssignment(ctx context.Context, flagID uuid.UUID, subjectID string) (*Assignment, error) {
	args := m.Called(ctx, flagID, subjectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Assignment), args.Error(1)
}

func (m *MockAssignmentRepository) UpsertAssignment(ctx context.Context, assignment *Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

type MockExperimentRepository struct {
	mock.Mock
}

func (m *MockExperimentRepository) GetExperimentByFlagID(ctx context.Context, flagID uuid.UUID) (*Experiment, error) {
	args := m.Called(ctx, flagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Experiment), args.Error(1)
}

func (m *MockExperimentRepository) GetExperimentVariants(ctx context.Context, experimentID uuid.UUID) ([]*ExperimentVariant, error) {
	args := m.Called(ctx, experimentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ExperimentVariant), args.Error(1)
}

type MockExperimentEngine struct {
	mock.Mock
}

func (m *MockExperimentEngine) ProcessExperiment(ctx context.Context, flag *Flag, assignment *Assignment, exp *Experiment) (*ExperimentResult, error) {
	args := m.Called(ctx, flag, assignment, exp)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ExperimentResult), args.Error(1)
}

func TestFlagEvaluator_EvaluateFlag_DisabledFlag(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	// Create disabled flag
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  false,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	// No experiment for this flag
	mockExperimentRepo.On("GetExperimentByFlagID", mock.Anything, flagID).Return(nil, errors.New("no experiment"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "test-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`false`), result.Value)
	assert.Greater(t, result.Bucket, -1)
	assert.Less(t, result.Bucket, 10000)

	mockFlagRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_EnabledFlagWithRules(t *testing.T) {
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
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	// Create rules with different priorities
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 2,
			Rollout:  50,
			Variant:  json.RawMessage(`{"enabled": false}`),
		},
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1, // Higher priority (lower number)
			Rollout:  100,
			Variant:  json.RawMessage(`{"enabled": true}`),
		},
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "test-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`{"enabled": true}`), result.Value)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_StickyAssignment(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeJSON,
	}

	// Existing assignment
	existingAssignment := &Assignment{
		ID:            uuid.New(),
		FlagID:        flagID,
		SubjectID:     "user123",
		Bucket:        1234,
		ChosenVariant: json.RawMessage(`{"version": "v2"}`),
		AssignedAt:    time.Now(),
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(existingAssignment, nil)

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "test-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`{"version": "v2"}`), result.Value)
	assert.Equal(t, 1234, result.Bucket)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_NoMatchingRules(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	// Rules with 0% rollout (no one should match)
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  0, // 0% rollout
			Variant:  json.RawMessage(`true`),
		},
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert
	assert.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "test-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`false`), result.Value)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_SafeDefaultOnFlagNotFound(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "nonexistent-flag").Return(nil, errors.New("flag not found"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "nonexistent-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert - should return safe default, not error
	assert.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "nonexistent-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`false`), result.Value)
	assert.Equal(t, 0, result.Bucket)

	mockFlagRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_SafeDefaultOnRulesFailure(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(nil, errors.New("database error"))
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert - should return safe default, not error
	assert.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "test-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`false`), result.Value)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_RulePriorityOrdering(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeJSON,
	}

	// Rules in random order - evaluator should sort by priority
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 3,
			Rollout:  100,
			Variant:  json.RawMessage(`{"priority": 3}`),
		},
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1, // Highest priority
			Rollout:  100,
			Variant:  json.RawMessage(`{"priority": 1}`),
		},
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 2,
			Rollout:  100,
			Variant:  json.RawMessage(`{"priority": 2}`),
		},
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert - should match the highest priority rule (priority 1)
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, json.RawMessage(`{"priority": 1}`), result.Value)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_AssignmentPersistenceFailure(t *testing.T) {
	// Setup
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  100,
			Variant:  json.RawMessage(`true`),
		},
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "user123").Return(nil, errors.New("not found"))
	// Assignment persistence fails, but evaluation should still succeed
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(errors.New("database error"))

	// Execute
	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "user123",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	// Assert - evaluation should succeed even if assignment persistence fails
	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, json.RawMessage(`true`), result.Value)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

// Additional comprehensive tests for requirement coverage

func TestFlagEvaluator_EvaluateFlag_DeterministicBehavior(t *testing.T) {
	// Test that the same subject gets the same result consistently (Requirement 1.1, 1.2)
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "test-flag",
		Enabled:  true,
		Salt:     "consistent-salt",
		Type:     FlagTypeBoolean,
	}

	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  100, // 100% rollout to ensure consistent matching
			Variant:  json.RawMessage(`true`),
		},
	}

	// First evaluation - no existing assignment
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil).Once()
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil).Once()
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "consistent-user").Return(nil, errors.New("not found")).Once()
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil).Once()

	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "test-flag",
		SubjectID: "consistent-user",
	}

	// First evaluation
	result1, err1 := evaluator.EvaluateFlag(context.Background(), req)
	assert.NoError(t, err1)

	// Create the assignment that would be persisted
	assignment := &Assignment{
		ID:            uuid.New(),
		FlagID:        flagID,
		SubjectID:     "consistent-user",
		Bucket:        result1.Bucket,
		ChosenVariant: result1.Value,
		AssignedAt:    time.Now(),
	}

	// Second evaluation - should find existing assignment
	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "test-flag").Return(flag, nil).Once()
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "consistent-user").Return(assignment, nil).Once()

	// Second evaluation
	result2, err2 := evaluator.EvaluateFlag(context.Background(), req)
	assert.NoError(t, err2)

	// Results should be identical (deterministic)
	assert.Equal(t, result1.Enabled, result2.Enabled)
	assert.Equal(t, result1.Bucket, result2.Bucket)
	assert.Equal(t, result1.Value, result2.Value)

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_MultipleRulesPriorityEvaluation(t *testing.T) {
	// Test that rules are evaluated in priority order (Requirement 1.3)
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "priority-test",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeJSON,
	}

	// Multiple rules with different priorities and rollouts
	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 10, // Lower priority
			Rollout:  100,
			Variant:  json.RawMessage(`{"rule": "low-priority"}`),
		},
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1, // Highest priority
			Rollout:  10, // Low rollout - might not match
			Variant:  json.RawMessage(`{"rule": "high-priority"}`),
		},
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 5, // Medium priority
			Rollout:  100,
			Variant:  json.RawMessage(`{"rule": "medium-priority"}`),
		},
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "priority-test").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "test-user").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)

	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "priority-test",
		SubjectID: "test-user",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	assert.NoError(t, err)
	
	// Should either match the highest priority rule (if in 10% rollout) 
	// or the medium priority rule (if not in 10% but in 100% rollout)
	// The key is that it should NEVER match the lowest priority rule
	// because medium priority has 100% rollout and comes before it
	if result.Enabled {
		value := string(result.Value)
		assert.True(t, 
			value == `{"rule": "high-priority"}` || value == `{"rule": "medium-priority"}`,
			"Should match either high or medium priority rule, got: %s", value)
		assert.NotEqual(t, `{"rule": "low-priority"}`, value, "Should never match low priority rule")
	}

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}

func TestFlagEvaluator_EvaluateFlag_SafeDefaultOnSystemFailure(t *testing.T) {
	// Test safe default handling for system failures (Requirement 1.4)
	testCases := []struct {
		name           string
		setupMocks     func(*MockFlagRepository, *MockAssignmentRepository)
		expectedResult *EvaluationResult
	}{
		{
			name: "flag repository failure",
			setupMocks: func(flagRepo *MockFlagRepository, assignmentRepo *MockAssignmentRepository) {
				flagRepo.On("GetFlagByKey", mock.Anything, mock.Anything, "test-flag").Return(nil, errors.New("database connection failed"))
			},
			expectedResult: &EvaluationResult{
				FlagKey: "test-flag",
				Enabled: false,
				Value:   json.RawMessage(`false`),
				Bucket:  0,
			},
		},
		{
			name: "rules repository failure",
			setupMocks: func(flagRepo *MockFlagRepository, assignmentRepo *MockAssignmentRepository) {
				flag := &Flag{
					ID:      uuid.New(),
					Key:     "test-flag",
					Enabled: true,
					Salt:    "test-salt",
				}
				flagRepo.On("GetFlagByKey", mock.Anything, mock.Anything, "test-flag").Return(flag, nil)
				flagRepo.On("GetFlagRules", mock.Anything, mock.Anything).Return(nil, errors.New("rules query failed"))
				assignmentRepo.On("GetAssignment", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
			},
			expectedResult: &EvaluationResult{
				FlagKey: "test-flag",
				Enabled: false,
				Value:   json.RawMessage(`false`),
				Bucket:  0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockFlagRepo := new(MockFlagRepository)
			mockAssignmentRepo := new(MockAssignmentRepository)
			mockExperimentRepo := new(MockExperimentRepository)
			mockExperimentEngine := new(MockExperimentEngine)
			evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

			tc.setupMocks(mockFlagRepo, mockAssignmentRepo)

			req := EvaluationRequest{
				TenantID:  uuid.New(),
				FlagKey:   "test-flag",
				SubjectID: "test-user",
			}

			result, err := evaluator.EvaluateFlag(context.Background(), req)

			// Should not return error, but safe default instead
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResult.FlagKey, result.FlagKey)
			assert.Equal(t, tc.expectedResult.Enabled, result.Enabled)
			assert.Equal(t, tc.expectedResult.Value, result.Value)

			mockFlagRepo.AssertExpectations(t)
			mockAssignmentRepo.AssertExpectations(t)
		})
	}
}

func TestFlagEvaluator_EvaluateFlag_DisabledFlagBehavior(t *testing.T) {
	// Test that disabled flags return false regardless of rules (Requirement 1.5)
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	// Disabled flag
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "disabled-flag",
		Enabled:  false, // Disabled
		Salt:     "test-salt",
		Type:     FlagTypeBoolean,
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "disabled-flag").Return(flag, nil)
	// Note: We don't expect GetFlagRules to be called for disabled flags

	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "disabled-flag",
		SubjectID: "any-user",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	assert.NoError(t, err)
	assert.False(t, result.Enabled)
	assert.Equal(t, "disabled-flag", result.FlagKey)
	assert.Equal(t, json.RawMessage(`false`), result.Value)
	assert.Greater(t, result.Bucket, -1) // Bucket should still be calculated
	assert.Less(t, result.Bucket, 10000)

	mockFlagRepo.AssertExpectations(t)
	// Assignment repo should not be called for disabled flags
	mockAssignmentRepo.AssertNotCalled(t, "GetAssignment")
	mockAssignmentRepo.AssertNotCalled(t, "UpsertAssignment")
}

func TestFlagEvaluator_EvaluateFlag_JSONFlagType(t *testing.T) {
	// Test JSON flag evaluation
	mockFlagRepo := new(MockFlagRepository)
	mockAssignmentRepo := new(MockAssignmentRepository)
	mockExperimentRepo := new(MockExperimentRepository)
	mockExperimentEngine := new(MockExperimentEngine)
	evaluator := NewFlagEvaluator(mockFlagRepo, mockAssignmentRepo, mockExperimentRepo, mockExperimentEngine)

	tenantID := uuid.New()
	flagID := uuid.New()
	
	flag := &Flag{
		ID:       flagID,
		TenantID: tenantID,
		Key:      "json-flag",
		Enabled:  true,
		Salt:     "test-salt",
		Type:     FlagTypeJSON,
	}

	rules := []*FlagRule{
		{
			ID:       uuid.New(),
			FlagID:   flagID,
			Priority: 1,
			Rollout:  100,
			Variant:  json.RawMessage(`{"feature": "enabled", "config": {"timeout": 5000, "retries": 3}}`),
		},
	}

	mockFlagRepo.On("GetFlagByKey", mock.Anything, tenantID, "json-flag").Return(flag, nil)
	mockFlagRepo.On("GetFlagRules", mock.Anything, flagID).Return(rules, nil)
	mockAssignmentRepo.On("GetAssignment", mock.Anything, flagID, "json-user").Return(nil, errors.New("not found"))
	mockAssignmentRepo.On("UpsertAssignment", mock.Anything, mock.AnythingOfType("*flags.Assignment")).Return(nil)

	req := EvaluationRequest{
		TenantID:  tenantID,
		FlagKey:   "json-flag",
		SubjectID: "json-user",
	}

	result, err := evaluator.EvaluateFlag(context.Background(), req)

	assert.NoError(t, err)
	assert.True(t, result.Enabled)
	assert.Equal(t, "json-flag", result.FlagKey)
	
	// Verify JSON structure is preserved
	expectedJSON := json.RawMessage(`{"feature": "enabled", "config": {"timeout": 5000, "retries": 3}}`)
	assert.JSONEq(t, string(expectedJSON), string(result.Value))

	mockFlagRepo.AssertExpectations(t)
	mockAssignmentRepo.AssertExpectations(t)
}