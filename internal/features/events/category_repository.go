package events

import (
	"context"
	"errors"

	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/repository/sql"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/core/audit"
)

const eventCategoriesTable = "event_categories"

// CategoryRepository defines persistence for event categories.
// Soft-delete, audit field management, and deleted_at filtering are handled
// by the underlying AuditableRepository.
type CategoryRepository interface {
	Create(ctx context.Context, entity *EventCategory) error
	GetByID(ctx context.Context, id any) (*EventCategory, error)
	Update(ctx context.Context, id any, entity *EventCategory) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, opts *repository.ListOptions) ([]*EventCategory, int64, error)
	Count(ctx context.Context, filter repository.Filter) (int64, error)
	Exists(ctx context.Context, id any) (bool, error)
}

// CategoryRepositoryOptions holds configuration for the category repository.
type CategoryRepositoryOptions struct{}

// categoryRepo implements CategoryRepository.
// Delegates to an AuditableRepository which wraps the SQL repository.
type categoryRepo struct {
	repo    repository.Repository[EventCategory, string]
	options CategoryRepositoryOptions
}

// NewCategoryRepository returns a CategoryRepository backed by AuditableRepository.
func NewCategoryRepository(options CategoryRepositoryOptions, log logger.Logger, db *sqlkit.DB) CategoryRepository {
	sqlRepo := sql.NewSQLRepository[EventCategory, string](
		log,
		db,
		eventCategoriesTable,
		sql.WithSelectColumns[EventCategory, string]([]string{
			"id", "source", "tenant_id", "name", "created_at", "updated_at", "deleted_at",
		}),
	)
	auditableRepo := audit.NewAuditableRepository[EventCategory, string](sqlRepo)
	return &categoryRepo{repo: auditableRepo, options: options}
}

func (r *categoryRepo) Create(ctx context.Context, entity *EventCategory) error {
	return r.repo.Create(ctx, entity)
}

func (r *categoryRepo) GetByID(ctx context.Context, id any) (*EventCategory, error) {
	idStr, _ := id.(string)
	return r.repo.GetByID(ctx, idStr)
}

func (r *categoryRepo) Update(ctx context.Context, id any, entity *EventCategory) error {
	idStr, _ := id.(string)
	return r.repo.Update(ctx, idStr, entity)
}

func (r *categoryRepo) Delete(ctx context.Context, id any) error {
	idStr, _ := id.(string)
	return r.repo.Delete(ctx, idStr)
}

func (r *categoryRepo) List(ctx context.Context, opts *repository.ListOptions) ([]*EventCategory, int64, error) {
	return r.repo.List(ctx, opts)
}

func (r *categoryRepo) Count(ctx context.Context, filter repository.Filter) (int64, error) {
	return r.repo.Count(ctx, filter)
}

func (r *categoryRepo) Exists(ctx context.Context, id any) (bool, error) {
	_, err := r.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
