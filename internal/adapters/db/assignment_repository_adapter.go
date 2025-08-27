package db

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgtype"
    "github.com/latoulicious/RoutePilot/internal/domain/flags"
    "github.com/latoulicious/RoutePilot/internal/ports"
)

// AssignmentRepositoryAdapter adapts the database repository for assignment operations
type AssignmentRepositoryAdapter struct {
    queries *Queries
}

var _ ports.AssignmentRepository = (*AssignmentRepositoryAdapter)(nil)

func NewAssignmentRepositoryAdapter(queries *Queries) *AssignmentRepositoryAdapter {
    return &AssignmentRepositoryAdapter{queries: queries}
}

func (r *AssignmentRepositoryAdapter) GetAssignment(ctx context.Context, flagID uuid.UUID, subjectID string) (*flags.Assignment, error) {
    var pgFlagID pgtype.UUID
    if err := pgFlagID.Scan(flagID); err != nil {
        return nil, fmt.Errorf("invalid flag ID: %v", err)
    }

    dbAssign, err := r.queries.GetAssignment(ctx, GetAssignmentParams{FlagID: pgFlagID, SubjectID: subjectID})
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("assignment not found")
        }
        return nil, fmt.Errorf("failed to get assignment: %v", err)
    }

    return r.convertDBAssignmentToCore(dbAssign)
}

func (r *AssignmentRepositoryAdapter) UpsertAssignment(ctx context.Context, assignment *flags.Assignment) error {
    var pgFlagID pgtype.UUID
    if err := pgFlagID.Scan(assignment.FlagID); err != nil {
        return fmt.Errorf("invalid flag ID: %v", err)
    }

    params := UpsertAssignmentParams{
        FlagID:        pgFlagID,
        SubjectID:     assignment.SubjectID,
        Bucket:        int32(assignment.Bucket),
        ChosenVariant: []byte(assignment.ChosenVariant),
    }

    dbAssign, err := r.queries.UpsertAssignment(ctx, params)
    if err != nil {
        return fmt.Errorf("failed to upsert assignment: %v", err)
    }

    // update provided model with returned values
    coreAssign, err := r.convertDBAssignmentToCore(dbAssign)
    if err != nil {
        return err
    }
    *assignment = *coreAssign
    return nil
}

func (r *AssignmentRepositoryAdapter) convertDBAssignmentToCore(dbA FlagAssignment) (*flags.Assignment, error) {
    id, err := uuid.FromBytes(dbA.ID.Bytes[:])
    if err != nil {
        return nil, fmt.Errorf("failed to convert assignment ID: %v", err)
    }
    flagID, err := uuid.FromBytes(dbA.FlagID.Bytes[:])
    if err != nil {
        return nil, fmt.Errorf("failed to convert flag ID: %v", err)
    }

    var variant json.RawMessage
    if len(dbA.ChosenVariant) > 0 {
        variant = json.RawMessage(dbA.ChosenVariant)
    }

    a := &flags.Assignment{
        ID:            id,
        FlagID:        flagID,
        SubjectID:     dbA.SubjectID,
        Bucket:        int(dbA.Bucket),
        ChosenVariant: variant,
    }
    if dbA.AssignedAt.Valid {
        a.AssignedAt = dbA.AssignedAt.Time
    }
    return a, nil
}

