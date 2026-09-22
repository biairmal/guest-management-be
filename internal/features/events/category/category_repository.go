package category

import (
	"context"
	"fmt"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/repository"
	reposql "github.com/biairmal/go-sdk/lib/repository/sql"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/google/uuid"

	corerepository "github.com/biairmal/guest-management-be/internal/core/repository"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../../mocks/events/category/mock_template_version_repository.go -package=mockcategory github.com/biairmal/guest-management-be/internal/features/events/category TemplateVersionRepository

const eventCategoriesTable = "event_categories"

// eventCategoryColumns are the columns selected on reads (GetByID, List).
var eventCategoryColumns = []string{
	"id", "source", "tenant_id", "name", "template_version", "created_at", "updated_at", "deleted_at",
}

// NewCategoryRepository returns a soft-delete-aware repository for event categories.
// TID is uuid.UUID — kept typed all the way through the service layer.
func NewCategoryRepository(
	log logger.Logger, db *sqlkit.DB, cacheOpts corerepository.CacheOptions,
) repository.Repository[EventCategory, uuid.UUID] {
	return corerepository.NewRepository[EventCategory, uuid.UUID](
		log, db, eventCategoriesTable, eventCategoryColumns, cacheOpts,
	)
}

// TemplateVersionRepository reads a live category's template_version under
// a row lock held until ctx's transaction ends. It reads the database
// directly, never the category cache, so the version is the committed one.
// Both methods return repository.ErrNotFound for a missing or soft-deleted
// category.
type TemplateVersionRepository interface {
	// TemplateVersionForUpdate locks the row exclusively (FOR UPDATE):
	// concurrent template saves serialize, and the later one sees the new
	// version and fails its compare.
	TemplateVersionForUpdate(ctx context.Context, id uuid.UUID) (int, error)
	// TemplateVersionForShare locks the row FOR SHARE: a template save
	// waits for the event creation that copies this version to commit.
	TemplateVersionForShare(ctx context.Context, id uuid.UUID) (int, error)
}

// sqlTemplateVersionRepository implements TemplateVersionRepository with
// direct SQL on the transaction in ctx (leader when there is none).
type sqlTemplateVersionRepository struct {
	base *reposql.BaseRepository
	log  logger.Logger
}

// NewTemplateVersionRepository returns a TemplateVersionRepository backed by direct SQL.
func NewTemplateVersionRepository(log logger.Logger, db *sqlkit.DB) TemplateVersionRepository {
	return &sqlTemplateVersionRepository{base: reposql.NewBaseRepository(db, eventCategoriesTable), log: log}
}

// TemplateVersionForUpdate implements TemplateVersionRepository.
func (r *sqlTemplateVersionRepository) TemplateVersionForUpdate(ctx context.Context, id uuid.UUID) (int, error) {
	return r.lockedVersion(ctx, id, "FOR UPDATE")
}

// TemplateVersionForShare implements TemplateVersionRepository.
func (r *sqlTemplateVersionRepository) TemplateVersionForShare(ctx context.Context, id uuid.UUID) (int, error) {
	return r.lockedVersion(ctx, id, "FOR SHARE")
}

func (r *sqlTemplateVersionRepository) lockedVersion(ctx context.Context, id uuid.UUID, lock string) (int, error) {
	q := `SELECT template_version FROM event_categories WHERE id = $1 AND deleted_at IS NULL ` + lock
	if r.log != nil {
		r.log.DebugfWithContext(ctx, "query: %s args: %v", q, []any{id})
	}
	var version int
	err := r.base.GetConnection(ctx).QueryRowContext(ctx, q, id).Scan(&version)
	if sqlkit.IsNoRows(err) {
		return 0, repository.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("event_categories: read template version: %w", err)
	}
	return version, nil
}
