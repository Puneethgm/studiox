package messaging

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// RecordToolCallAudit writes one tool_call_audit row (task 7.4) —
// regardless of success/failure, since a failed call is exactly as
// important to have a record of as a successful one.
//
// JSON values are passed as string, not []byte — pgx binds a []byte
// parameter against a jsonb column as bytea, not jsonb, and errors; see
// Repo.RecordAlert's identical note on this gotcha.
func (r *Repo) RecordToolCallAudit(ctx context.Context, conversationID, studioID uuid.UUID, tool string, argumentsJSON, resultJSON []byte, errMsg *string) error {
	if len(argumentsJSON) == 0 {
		argumentsJSON = []byte("{}")
	}
	var resultArg any
	if len(resultJSON) > 0 {
		resultArg = string(resultJSON)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tool_call_audit (conversation_id, studio_id, tool, arguments, result, error)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, conversationID, studioID, tool, string(argumentsJSON), resultArg, errMsg)
	if err != nil {
		return fmt.Errorf("record tool call audit: %w", err)
	}
	return nil
}
