package scans

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
	"github.com/google/uuid"
)

const scanLogsTable = "scan_logs"

// scanLogColumns are the columns selected on reads (GetByID, List).
var scanLogColumns = []string{
	"id", "event_id", "ticket_id", "workflow_step_id", "scanned_at", "operator_user_id",
}

// NewScanLogRepository returns a repository for scan_logs. scan_logs has no
// deleted_at column (a permanent, append-only audit log — see ScanLog doc),
// so this uses corerepository.NewRepositoryNoAudit rather than NewRepository,
// the same call roles.NewRoleRepository already makes for its own
// no-deleted_at table.
//
// Deliberately never wrapped in the cache decorator (unlike every other
// feature repository): ScanLogService.RecordScan's non-repeatable-step
// duplicate check (AC2) calls Count on every scan and must see every prior
// write immediately. go-sdk's cache decorator caches List/Count results by
// filter key (see go-sdk/lib/repository/cache/decorator.go), which could let
// a stale count pass a step that was already completed. TID is uuid.UUID —
// kept typed all the way through the service layer.
func NewScanLogRepository(log logger.Logger, db *sqlkit.DB) repository.Repository[ScanLog, uuid.UUID] {
	return corerepository.NewRepositoryNoAudit[ScanLog, uuid.UUID](
		log, db, scanLogsTable, scanLogColumns, corerepository.CacheOptions{},
	)
}
