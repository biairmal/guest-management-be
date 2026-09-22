package category

import "github.com/google/uuid"

const (
	// SourceApp denotes a system-defined category (tenant_id must be nil).
	SourceApp = "app"
	// SourceTenant denotes a tenant-defined category (tenant_id required).
	SourceTenant = "tenant"

	// PermissionManageEvents is the permission code required to create,
	// replace, or delete an event category (seeded by migration 000014 as
	// "events and categories"). Declared here since permission codes are
	// feature-owned (AGENTS.md).
	PermissionManageEvents = "manage_events"
)

// PlatformTenantID is the seeded system tenant (migration 000014). Only a
// caller from this tenant may create or change app categories (source = app).
var PlatformTenantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
