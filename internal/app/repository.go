package app

import (
	"github.com/biairmal/go-sdk/logger"
	sdkrepository "github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/google/uuid"
)

// repositories holds all feature repositories wired for the application.
type repositories struct {
	categoryRepository sdkrepository.Repository[events.EventCategory, uuid.UUID]
}

func (a *App) initializeRepository(log logger.Logger, db *sqlkit.DB) *repositories {
	return &repositories{
		categoryRepository: events.NewCategoryRepository(log, db),
	}
}
