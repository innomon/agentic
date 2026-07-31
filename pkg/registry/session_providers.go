// internal/registry/session_providers.go
package registry

import (
	"context"
	"fmt"

	"github.com/innomon/agentic/pkg/gnogent/storage"
	"google.golang.org/adk/v2/session"
	sessiondb "google.golang.org/adk/v2/session/database"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openDialector(driver, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "postgres":
		return postgres.Open(dsn), nil
	case "sqlite":
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported driver %q (supported: postgres, sqlite)", driver)
	}
}

func databaseSessionCreator(ctx context.Context, cfg *SessionConfig) (session.Service, error) {
	dialector, err := openDialector(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to create session service: %w", err)
	}
	sessionSvc, err := sessiondb.NewSessionService(dialector)
	if err != nil {
		return nil, fmt.Errorf("failed to create session service: %w", err)
	}
	if cfg.AutoMigrate {
		if err := sessiondb.AutoMigrate(sessionSvc); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate session schema: %w", err)
		}
	}
	return sessionSvc, nil
}

func gnogentSessionCreator(ctx context.Context, cfg *SessionConfig) (session.Service, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	svc := storage.NewGnogentSessionService(db)
	if cfg.AutoMigrate {
		if err := svc.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	}
	return svc, nil
}

func init() {
	RegisterProvider("session", "database", ProviderCreator[SessionConfig, session.Service](databaseSessionCreator))
	RegisterProvider("session", "gnogent", ProviderCreator[SessionConfig, session.Service](gnogentSessionCreator))
}
