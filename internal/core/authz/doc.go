// Package authz is the reusable permission-check building block shared by
// every feature slice that needs role-based access control. It lives in
// internal/core (not inside a feature slice) because B6 staffing is only
// the first consumer — later phases (tickets/guests/scans) reuse the same
// Checker with their own permission codes rather than each slice
// reimplementing "does this caller's role hold this permission code".
//
// Checker is a small, pure permission oracle: it answers "does the role_id
// claim in ctx hold permission code X", nothing more. Tenant scoping
// (comparing an entity's tenant_id against the caller's tenant_id claim) is
// deliberately NOT part of this package — that is an ordinary business-rule
// check done per-feature in the service layer (see
// internal/features/staffing's loadTenantScopedEvent for the pattern), not
// an authorization primitive.
package authz
