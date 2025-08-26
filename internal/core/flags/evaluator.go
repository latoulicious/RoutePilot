package flags

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Repository interfaces for dependency injection
type FlagRepositoryInterface interface {
	GetFlagByKey(ctx context.Context, tenantID uuid.UUID, key string) (*Flag, error)
	GetFlagRules(ctx context.Context, flagID uuid.UUID) ([]*FlagRule, error)
}

type AssignmentRepositoryInterface interface {
	GetAssignment(ctx context.Context, flagID uuid.UUID, subjectID string) (*Assignment, error)
	UpsertAssignment(ctx context.Context, assignment *Assignment) error
}



// FlagEvaluator implements the Evaluator interface
type FlagEvaluator struct {
	flagRepo       FlagRepositoryInterface
	assignmentRepo AssignmentRepositoryInterface
	experimentRepo ExperimentRepositoryInterface
	experimentEngine ExperimentEngine
}

// NewFlagEvaluator creates a new flag evaluator
func NewFlagEvaluator(flagRepo FlagRepositoryInterface, assignmentRepo AssignmentRepositoryInterface, experimentRepo ExperimentRepositoryInterface, experimentEngine ExperimentEngine) *FlagEvaluator {
	return &FlagEvaluator{
		flagRepo:       flagRepo,
		assignmentRepo: assignmentRepo,
		experimentRepo: experimentRepo,
		experimentEngine: experimentEngine,
	}
}

// EvaluateFlag evaluates a flag for a given subject with sticky assignment logic
func (e *FlagEvaluator) EvaluateFlag(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error) {
	// Safe default result in case of failures
	safeDefault := &EvaluationResult{
		FlagKey: req.FlagKey,
		Enabled: false,
		Value:   json.RawMessage(`false`),
		Bucket:  0,
	}

	// Get the flag
	flag, err := e.flagRepo.GetFlagByKey(ctx, req.TenantID, req.FlagKey)
	if err != nil {
		// Return safe default on flag retrieval failure
		return safeDefault, nil
	}

	// If flag is disabled, return disabled result
	if !flag.Enabled {
		result := &EvaluationResult{
			FlagKey: req.FlagKey,
			Enabled: false,
			Value:   json.RawMessage(`false`),
			Bucket:  CalculateBucket(flag.Salt, req.SubjectID),
		}
		return result, nil
	}

	// Calculate bucket for this subject
	bucket := CalculateBucket(flag.Salt, req.SubjectID)

	// Check for existing assignment (sticky behavior)
	existingAssignment, err := e.assignmentRepo.GetAssignment(ctx, flag.ID, req.SubjectID)
	if err == nil && existingAssignment != nil {
		// Return existing sticky assignment
		return &EvaluationResult{
			FlagKey: req.FlagKey,
			Enabled: true,
			Value:   existingAssignment.ChosenVariant,
			Bucket:  existingAssignment.Bucket,
		}, nil
	}

	// Get flag rules
	rules, err := e.flagRepo.GetFlagRules(ctx, flag.ID)
	if err != nil {
		// Return safe default on rules retrieval failure
		return safeDefault, nil
	}

	// Sort rules by priority (ascending order - lower numbers have higher priority)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	// Evaluate rules in priority order
	var chosenVariant json.RawMessage
	var matched bool

	for _, rule := range rules {
		if IsInRollout(bucket, rule.Rollout) {
			chosenVariant = rule.Variant
			matched = true
			break
		}
	}

	// If no rule matched, return disabled
	if !matched {
		result := &EvaluationResult{
			FlagKey: req.FlagKey,
			Enabled: false,
			Value:   json.RawMessage(`false`),
			Bucket:  bucket,
		}
		return result, nil
	}

	// Create new assignment for sticky behavior
	assignment := &Assignment{
		ID:            uuid.New(),
		FlagID:        flag.ID,
		SubjectID:     req.SubjectID,
		Bucket:        bucket,
		ChosenVariant: chosenVariant,
		AssignedAt:    time.Now(),
	}

	// Persist assignment (best effort - don't fail evaluation if this fails)
	_ = e.assignmentRepo.UpsertAssignment(ctx, assignment)

	// Check for experiments on this flag
	experiment, err := e.experimentRepo.GetExperimentByFlagID(ctx, flag.ID)
	if err == nil && experiment != nil {
		// Process experiment
		expResult, expErr := e.experimentEngine.ProcessExperiment(ctx, flag, assignment, experiment)
		if expErr == nil && expResult != nil {
			// Return experiment result with experiment metadata
			result := &EvaluationResult{
				FlagKey:       req.FlagKey,
				Enabled:       true,
				Value:         expResult.Value,
				Bucket:        bucket,
				ExperimentKey: &experiment.Key,
			}
			
			if expResult.AssignedVariant != nil {
				result.VariantName = &expResult.AssignedVariant.Name
			}
			
			return result, nil
		}
		// If experiment processing fails, continue with normal flag evaluation
	}

	// Return successful evaluation result (no experiment or experiment failed)
	result := &EvaluationResult{
		FlagKey: req.FlagKey,
		Enabled: true,
		Value:   chosenVariant,
		Bucket:  bucket,
	}

	return result, nil
}

// EvaluationError represents different types of evaluation errors
type EvaluationError struct {
	Type    string
	Message string
	Cause   error
}

func (e *EvaluationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Error types
var (
	ErrFlagNotFound     = errors.New("flag not found")
	ErrInvalidTenant    = errors.New("invalid tenant")
	ErrInvalidSubject   = errors.New("invalid subject ID")
	ErrSystemFailure    = errors.New("system failure")
)

// NewEvaluationError creates a new evaluation error
func NewEvaluationError(errorType, message string, cause error) *EvaluationError {
	return &EvaluationError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}