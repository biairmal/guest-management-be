package events

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

const (
	// SourceApp denotes a system-defined category (tenant_id must be nil).
	SourceApp = "app"
	// SourceTenant denotes a tenant-defined category (tenant_id required).
	SourceTenant = "tenant"
)

// EventCategory represents a row in the event_categories table.
// Supports soft delete via deleted_at.
type EventCategory struct {
	ID        uuid.UUID  `json:"id"`
	Source    string     `json:"source"`
	TenantID  *uuid.UUID `json:"tenant_id,omitempty"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// TableName returns the database table name.
func (EventCategory) TableName() string {
	return "event_categories"
}

// ScanEventCategory scans a single row from sql.Rows into *EventCategory.
// Used by the generic repository as ScanFunc.
// UUID columns are scanned as string for driver compatibility (lib/pq, pgx).
func ScanEventCategory(rows *sql.Rows) (*EventCategory, error) {
	var c EventCategory
	var idStr, tenantIDStr sql.NullString
	var deletedAt sql.NullTime
	err := rows.Scan(
		&idStr,
		&c.Source,
		&tenantIDStr,
		&c.Name,
		&c.CreatedAt,
		&c.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}
	if idStr.Valid && idStr.String != "" {
		c.ID, err = uuid.Parse(idStr.String)
		if err != nil {
			return nil, err
		}
	}
	if tenantIDStr.Valid && tenantIDStr.String != "" {
		tid, err := uuid.Parse(tenantIDStr.String)
		if err != nil {
			return nil, err
		}
		c.TenantID = &tid
	}
	if deletedAt.Valid {
		c.DeletedAt = &deletedAt.Time
	}
	return &c, nil
}

// eventCategoryInsertBuilder implements sql.InsertQueryBuilder[EventCategory].
type eventCategoryInsertBuilder struct{}

func (eventCategoryInsertBuilder) BuildQuery(tableName, _ string) string {
	return `INSERT INTO ` +
		tableName +
		` (id, source, tenant_id, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)`
}

func (eventCategoryInsertBuilder) ExtractValues(c *EventCategory) []any {
	var tenantID any
	if c.TenantID != nil {
		tenantID = c.TenantID.String()
	}
	now := time.Now()
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = now
	}
	return []any{c.ID.String(), c.Source, tenantID, c.Name, c.CreatedAt, c.UpdatedAt}
}

func (eventCategoryInsertBuilder) SetID(_ *EventCategory, _ int64) error {
	// PostgreSQL UUID is not set via LastInsertId; ID is set by caller before Create.
	return nil
}

// eventCategoryUpdateBuilder implements sql.UpdateQueryBuilder[EventCategory].
type eventCategoryUpdateBuilder struct{}

func (eventCategoryUpdateBuilder) BuildQuery(_, _ string) []string {
	return []string{"source = $1", "tenant_id = $2", "name = $3", "updated_at = $4", "deleted_at = $5"}
}

func (eventCategoryUpdateBuilder) ExtractValues(c *EventCategory) []any {
	var tenantID any
	if c.TenantID != nil {
		tenantID = c.TenantID.String()
	}
	var deletedAt any
	if c.DeletedAt != nil {
		deletedAt = c.DeletedAt
	}
	return []any{c.Source, tenantID, c.Name, c.UpdatedAt, deletedAt}
}
