// internal/registry/memory_providers.go
package registry

import (
	"context"
	"fmt"

	"github.com/innomon/agentic/internal/gnogent/storage"
	"github.com/innomon/agentic/internal/memory"
	adkmemory "google.golang.org/adk/memory"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func databaseMemoryCreator(ctx context.Context, cfg *MemoryConfig) (adkmemory.Service, error) {
	return memory.OpenDatabaseMemoryService(cfg.Driver, cfg.DSN, cfg.AutoMigrate)
}

func gnogentMemoryCreator(ctx context.Context, cfg *MemoryConfig) (adkmemory.Service, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	svc := storage.NewGnogentMemoryService(db)
	if cfg.AutoMigrate {
		if err := svc.AutoMigrate(); err != nil {
			return nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	}
	return svc, nil
}

func init() {
	RegisterProvider("memory", "database", ProviderCreator[MemoryConfig, adkmemory.Service](databaseMemoryCreator))
	RegisterProvider("memory", "gnogent", ProviderCreator[MemoryConfig, adkmemory.Service](gnogentMemoryCreator))
}
