package events

import (
	"context"
	"errors"
	"time"

	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/repository/sql"
	"github.com/biairmal/go-sdk/sqlkit"
)

const eventCategoriesTable = "event_categories"

// CategoryRepository defines persistence for event categories.
// Uses soft delete: List/GetByID exclude deleted; Delete sets deleted_at.
type CategoryRepository interface {
	Create(ctx context.Context, entity *EventCategory) error
	GetByID(ctx context.Context, id any) (*EventCategory, error)
	Update(ctx context.Context, id any, entity *EventCategory) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, opts *repository.ListOptions) ([]*EventCategory, error)
	Count(ctx context.Context, filter repository.Filter) (int64, error)
	Exists(ctx context.Context, id any) (bool, error)
}

type CategoryRepositoryOptions struct{}

// categoryRepo implements CategoryRepository with soft-delete semantics.
type categoryRepo struct {
	repo    repository.Repository[EventCategory]
	options CategoryRepositoryOptions
}

// NewCategoryRepository returns a CategoryRepository backed by the generic SQL repository.
func NewCategoryRepository(options CategoryRepositoryOptions, db *sqlkit.DB) CategoryRepository {
	generic := sql.NewGenericRepository(
		db,
		eventCategoriesTable,
		ScanEventCategory,
		sql.WithInsertBuilder(eventCategoryInsertBuilder{}),
		sql.WithUpdateBuilder(eventCategoryUpdateBuilder{}),
		sql.WithAllowedColumns[EventCategory]([]string{
			"id", "source", "tenant_id", "name", "created_at", "updated_at", "deleted_at",
		}),
	)
	return &categoryRepo{repo: generic, options: options}
}

// mergeSoftDeleteFilter returns a filter that adds deleted_at IS NULL to the given filter.
func mergeSoftDeleteFilter(f repository.Filter) repository.Filter {
	if f.Conditions == nil {
		f.Conditions = make(map[string]any)
	}
	if f.RawWhere != "" {
		f.RawWhere += " AND deleted_at IS NULL"
	} else {
		f.RawWhere = "deleted_at IS NULL"
		f.RawArgs = nil
	}
	return f
}

func (r *categoryRepo) Create(ctx context.Context, entity *EventCategory) error {
	return r.repo.Create(ctx, entity)
}

func (r *categoryRepo) GetByID(ctx context.Context, id any) (*EventCategory, error) {
	entity, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entity.DeletedAt != nil {
		return nil, repository.ErrNotFound
	}
	return entity, nil
}

func (r *categoryRepo) Update(ctx context.Context, id any, entity *EventCategory) error {
	return r.repo.Update(ctx, id, entity)
}

func (r *categoryRepo) Delete(ctx context.Context, id any) error {
	entity, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entity.DeletedAt != nil {
		return repository.ErrNotFound
	}
	now := time.Now()
	entity.DeletedAt = &now
	entity.UpdatedAt = now
	return r.repo.Update(ctx, id, entity)
}

func (r *categoryRepo) List(ctx context.Context, opts *repository.ListOptions) ([]*EventCategory, error) {
	if opts == nil {
		opts = &repository.ListOptions{}
	}
	opts = &repository.ListOptions{
		Filter:     mergeSoftDeleteFilter(opts.Filter),
		Pagination: opts.Pagination,
		Sort:       opts.Sort,
	}
	return r.repo.List(ctx, opts)
}

func (r *categoryRepo) Count(ctx context.Context, filter repository.Filter) (int64, error) {
	return r.repo.Count(ctx, mergeSoftDeleteFilter(filter))
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
