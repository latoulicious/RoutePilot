package flags

import (
    "encoding/json"

    "github.com/google/uuid"
)

// EvaluationRequest represents a request to evaluate a flag
type EvaluationRequest struct {
    TenantID  uuid.UUID `json:"tenant_id"`
    FlagKey   string    `json:"flag_key"`
    SubjectID string    `json:"subject_id"`
}

// EvaluationResult represents the result of flag evaluation
type EvaluationResult struct {
    FlagKey       string          `json:"flag_key"`
    Enabled       bool            `json:"enabled"`
    Value         json.RawMessage `json:"value"`
    Bucket        int             `json:"bucket"`
    ExperimentKey *string         `json:"experiment_key,omitempty"`
    VariantName   *string         `json:"variant_name,omitempty"`
}

// ExperimentResult represents the result of experiment processing
type ExperimentResult struct {
    AssignedVariant *ExperimentVariant `json:"assigned_variant,omitempty"`
    Value           json.RawMessage    `json:"value"`
}

// FlagUpdates represents updates to a flag
type FlagUpdates struct {
    Description *string   `json:"description,omitempty"`
    Enabled     *bool     `json:"enabled,omitempty"`
    Salt        *string   `json:"salt,omitempty"`
}

