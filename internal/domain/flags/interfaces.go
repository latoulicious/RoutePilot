package flags

import (
    "context"
)

// Evaluator defines the interface for flag evaluation
type Evaluator interface {
    EvaluateFlag(ctx context.Context, req EvaluationRequest) (*EvaluationResult, error)
}

// ExperimentEngine defines the interface for experiment processing
type ExperimentEngine interface {
    ProcessExperiment(ctx context.Context, flag *Flag, assignment *Assignment, exp *Experiment) (*ExperimentResult, error)
}

