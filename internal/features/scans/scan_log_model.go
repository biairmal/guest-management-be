package scans

import (
	"time"

	"github.com/google/uuid"
)

// ScanLog represents a row in the scan_logs table — a permanent, append-only
// record of one ticket scan against one workflow step. There is no
// Update/Delete surface: once recorded, a scan is never edited or removed
// (see ScanLogService). OperatorUserID is always nil this phase — see
// docs/DEVELOPMENT_PLAN.md B9 non-goals ("no operator_user_id population").
// No soft delete (see scan_log_repository.go). Uses db tags for
// reflection-based scanning.
//
// swagger:model ScanLog
type ScanLog struct {
	ID             uuid.UUID  `json:"id"                         db:"id"`
	EventID        uuid.UUID  `json:"event_id"                   db:"event_id"`
	TicketID       uuid.UUID  `json:"ticket_id"                  db:"ticket_id"`
	WorkflowStepID uuid.UUID  `json:"workflow_step_id"           db:"workflow_step_id"`
	ScannedAt      time.Time  `json:"scanned_at"                 db:"scanned_at"`
	OperatorUserID *uuid.UUID `json:"operator_user_id,omitempty" db:"operator_user_id"`
}

// TableName returns the database table name.
func (ScanLog) TableName() string {
	return "scan_logs"
}
