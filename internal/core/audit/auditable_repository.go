package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/biairmal/go-sdk/repository"
)

// AuditableRepository wraps a repository.Repository and adds audit field management.
//
// Behaviour:
//   - Create: sets created_at and updated_at to time.Now(), then delegates to inner repo.
//   - Update: sets updated_at to time.Now(), then delegates to inner repo.
//   - Delete: performs soft-delete by setting deleted_at and updated_at to time.Now(),
//     then calls inner repo's Update (not Delete).
//   - GetByID: delegates to inner repo and returns ErrNotFound if entity is soft-deleted.
//   - List / Count: merges a "deleted_at IS NULL" condition into the filter before delegating.
//   - Exists: delegates to GetByID (respects soft-delete).
//
// The entity type must have struct fields with db tags: "created_at" (time.Time),
// "updated_at" (time.Time), and "deleted_at" (*time.Time).
type AuditableRepository[TEntity any, TID comparable] struct {
	inner repository.Repository[TEntity, TID]
}

// NewAuditableRepository creates a new AuditableRepository wrapping the given repository.
func NewAuditableRepository[TEntity any, TID comparable](
	inner repository.Repository[TEntity, TID],
) repository.Repository[TEntity, TID] {
	return &AuditableRepository[TEntity, TID]{inner: inner}
}

// Create sets created_at and updated_at, then delegates to the inner repository.
func (r *AuditableRepository[TEntity, TID]) Create(ctx context.Context, entity *TEntity) error {
	now := time.Now()
	setTimeField(entity, "created_at", now)
	setTimeField(entity, "updated_at", now)
	return r.inner.Create(ctx, entity)
}

// GetByID retrieves an entity and returns ErrNotFound if it has been soft-deleted.
func (r *AuditableRepository[TEntity, TID]) GetByID(ctx context.Context, id TID) (*TEntity, error) {
	entity, err := r.inner.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if isSoftDeleted(entity) {
		return nil, repository.ErrNotFound
	}
	return entity, nil
}

// Update sets updated_at to time.Now(), then delegates to the inner repository.
func (r *AuditableRepository[TEntity, TID]) Update(ctx context.Context, id TID, entity *TEntity) error {
	setTimeField(entity, "updated_at", time.Now())
	return r.inner.Update(ctx, id, entity)
}

// Delete performs a soft-delete: reads the entity, sets deleted_at and updated_at,
// then persists via the inner repository's Update method.
func (r *AuditableRepository[TEntity, TID]) Delete(ctx context.Context, id TID) error {
	entity, err := r.inner.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if isSoftDeleted(entity) {
		return repository.ErrNotFound
	}
	now := time.Now()
	setPtrTimeField(entity, "deleted_at", &now)
	setTimeField(entity, "updated_at", now)
	return r.inner.Update(ctx, id, entity)
}

// List merges a "deleted_at IS NULL" condition, then delegates to the inner repository.
func (r *AuditableRepository[TEntity, TID]) List(ctx context.Context, opts *repository.ListOptions) ([]*TEntity, int64, error) {
	if opts == nil {
		opts = &repository.ListOptions{}
	}
	merged := &repository.ListOptions{
		Filter:     appendSoftDeleteFilter(opts.Filter),
		Pagination: opts.Pagination,
		Sorts:      opts.Sorts,
		SkipCount:  opts.SkipCount,
	}
	return r.inner.List(ctx, merged)
}

// Count merges a "deleted_at IS NULL" condition, then delegates to the inner repository.
func (r *AuditableRepository[TEntity, TID]) Count(ctx context.Context, filter repository.Filter) (int64, error) {
	return r.inner.Count(ctx, appendSoftDeleteFilter(filter))
}

// Exists checks whether a non-deleted entity with the given ID exists.
func (r *AuditableRepository[TEntity, TID]) Exists(ctx context.Context, id TID) (bool, error) {
	_, err := r.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

var timeType = reflect.TypeOf(time.Time{})

// appendSoftDeleteFilter adds a "deleted_at IS NULL" condition to the filter.
func appendSoftDeleteFilter(f repository.Filter) repository.Filter {
	f.Conditions = append(f.Conditions, repository.FilterCondition{
		Field:    "deleted_at",
		Operator: repository.FilterOperatorIsNull,
	})
	return f
}

// setTimeField sets a time.Time field on the entity identified by the given db tag.
func setTimeField[T any](entity *T, dbTag string, value time.Time) {
	if entity == nil {
		return
	}
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		col := dbColumnName(t.Field(i))
		if col != dbTag {
			continue
		}
		field := v.Field(i)
		if field.CanSet() && field.Type() == timeType {
			field.Set(reflect.ValueOf(value))
		}
		return
	}
}

// setPtrTimeField sets a *time.Time field on the entity identified by the given db tag.
func setPtrTimeField[T any](entity *T, dbTag string, value *time.Time) {
	if entity == nil {
		return
	}
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		col := dbColumnName(t.Field(i))
		if col != dbTag {
			continue
		}
		field := v.Field(i)
		if field.CanSet() && field.Kind() == reflect.Ptr && field.Type().Elem() == timeType {
			field.Set(reflect.ValueOf(value))
		}
		return
	}
}

// isSoftDeleted returns true if the entity's deleted_at field (*time.Time) is non-nil.
func isSoftDeleted[T any](entity *T) bool {
	if entity == nil {
		return false
	}
	v := reflect.ValueOf(entity).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		col := dbColumnName(t.Field(i))
		if col != "deleted_at" {
			continue
		}
		field := v.Field(i)
		if field.Kind() == reflect.Ptr {
			return !field.IsNil()
		}
		return false
	}
	return false
}

// dbColumnName extracts the column name from a struct field's "db" tag.
// Returns "" if the tag is absent or "-".
func dbColumnName(f reflect.StructField) string {
	tag := f.Tag.Get("db")
	if tag == "" || tag == "-" {
		return ""
	}
	name := strings.TrimSpace(tag)
	if idx := strings.Index(name, ","); idx >= 0 {
		name = strings.TrimSpace(name[:idx])
	}
	return name
}
