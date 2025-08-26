package flags

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ExperimentRepositoryInterface defines the interface for experiment data access
type ExperimentRepositoryInterface interface {
	GetExperimentByFlagID(ctx context.Context, flagID uuid.UUID) (*Experiment, error)
	GetExperimentVariants(ctx context.Context, experimentID uuid.UUID) ([]*ExperimentVariant, error)
}

// ExperimentEngineImpl implements the ExperimentEngine interface
type ExperimentEngineImpl struct {
	experimentRepo ExperimentRepositoryInterface
	assignmentRepo AssignmentRepositoryInterface
}

// NewExperimentEngine creates a new experiment engine
func NewExperimentEngine(experimentRepo ExperimentRepositoryInterface, assignmentRepo AssignmentRepositoryInterface) *ExperimentEngineImpl {
	return &ExperimentEngineImpl{
		experimentRepo: experimentRepo,
		assignmentRepo: assignmentRepo,
	}
}

// ProcessExperiment processes an experiment for a given flag and assignment
func (e *ExperimentEngineImpl) ProcessExperiment(ctx context.Context, flag *Flag, assignment *Assignment, exp *Experiment) (*ExperimentResult, error) {
	// Check if experiment is in a valid status for assignment
	if !e.isExperimentActive(exp) {
		// Return the original flag value if experiment is not active
		return &ExperimentResult{
			Value: assignment.ChosenVariant,
		}, nil
	}

	// Check if subject qualifies for experiment traffic gating
	if !e.isSubjectInTraffic(flag.Salt, assignment.SubjectID, exp.Traffic) {
		// Subject not in experiment traffic, return original flag value
		return &ExperimentResult{
			Value: assignment.ChosenVariant,
		}, nil
	}

	// Get experiment variants
	variants, err := e.experimentRepo.GetExperimentVariants(ctx, exp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get experiment variants: %w", err)
	}

	if len(variants) == 0 {
		// No variants available, return original flag value
		return &ExperimentResult{
			Value: assignment.ChosenVariant,
		}, nil
	}

	// Check for existing experiment assignment (sticky behavior)
	existingAssignment, err := e.assignmentRepo.GetAssignment(ctx, flag.ID, assignment.SubjectID)
	if err == nil && existingAssignment != nil {
		// Find the variant that matches the existing assignment
		for _, variant := range variants {
			if string(existingAssignment.ChosenVariant) == string(variant.Value) {
				return &ExperimentResult{
					AssignedVariant: variant,
					Value:           variant.Value,
				}, nil
			}
		}
	}

	// Assign subject to a variant using weighted distribution
	selectedVariant := e.selectVariantByWeight(flag.Salt, assignment.SubjectID, variants)
	if selectedVariant == nil {
		// No variant selected, return original flag value
		return &ExperimentResult{
			Value: assignment.ChosenVariant,
		}, nil
	}

	// Create new experiment assignment for sticky behavior
	experimentAssignment := &Assignment{
		ID:            uuid.New(),
		FlagID:        flag.ID,
		SubjectID:     assignment.SubjectID,
		Bucket:        assignment.Bucket,
		ChosenVariant: selectedVariant.Value,
		AssignedAt:    time.Now(),
	}

	// Persist experiment assignment (best effort)
	_ = e.assignmentRepo.UpsertAssignment(ctx, experimentAssignment)

	return &ExperimentResult{
		AssignedVariant: selectedVariant,
		Value:           selectedVariant.Value,
	}, nil
}

// isExperimentActive checks if an experiment is in a status that allows new assignments
func (e *ExperimentEngineImpl) isExperimentActive(exp *Experiment) bool {
	return exp.Status == ExperimentStatusRunning
}

// isSubjectInTraffic determines if a subject qualifies for experiment traffic gating
func (e *ExperimentEngineImpl) isSubjectInTraffic(salt, subjectID string, trafficPercentage int) bool {
	if trafficPercentage <= 0 {
		return false
	}
	if trafficPercentage >= 100 {
		return true
	}

	// Use a different salt prefix for traffic gating to ensure independence from flag evaluation
	trafficBucket := CalculateBucket("traffic:"+salt, subjectID)
	trafficThreshold := trafficPercentage * 100 // Convert percentage to bucket range (0-9999)
	
	return trafficBucket < trafficThreshold
}

// selectVariantByWeight selects a variant based on weighted distribution
func (e *ExperimentEngineImpl) selectVariantByWeight(salt, subjectID string, variants []*ExperimentVariant) *ExperimentVariant {
	if len(variants) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, variant := range variants {
		totalWeight += variant.Weight
	}

	if totalWeight <= 0 {
		return nil
	}

	// Use deterministic selection based on subject ID and salt
	// Use a different salt prefix for variant selection to ensure independence
	variantBucket := CalculateBucket("variant:"+salt, subjectID)
	
	// Map bucket (0-9999) to weight range (0-totalWeight)
	selectedWeight := (variantBucket * totalWeight) / 10000

	// Find the variant that corresponds to the selected weight
	currentWeight := 0
	for _, variant := range variants {
		currentWeight += variant.Weight
		if selectedWeight < currentWeight {
			return variant
		}
	}

	// Fallback to last variant if rounding issues occur
	return variants[len(variants)-1]
}

// Experiment processing errors
var (
	ErrExperimentNotFound     = errors.New("experiment not found")
	ErrExperimentNotActive    = errors.New("experiment not active")
	ErrNoVariantsAvailable    = errors.New("no variants available")
	ErrInvalidVariantWeights  = errors.New("invalid variant weights")
)

// ExperimentError represents experiment-specific errors
type ExperimentError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ExperimentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// NewExperimentError creates a new experiment error
func NewExperimentError(errorType, message string, cause error) *ExperimentError {
	return &ExperimentError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}