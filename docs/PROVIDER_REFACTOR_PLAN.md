# Refactoring Plan: Provider Registry for Session and Memory Services

## 1. Introduction

The current implementation of `internal/registry/launcher.go:BuildLauncherConfig` uses hardcoded `switch/case` statements to instantiate `Session` and `Memory` service providers. This approach is rigid and requires code modification to add new provider types.

This plan details the refactoring process to replace the `switch/case` logic with a dynamic, registry-based system. This will align the provider implementation with the existing patterns for Models, Agents, and Tools, making the system more extensible and easier to maintain.

## 2. Core Refactoring Steps

The refactoring will be accomplished through the following steps:

1.  **Create a Generic Provider Framework:** A generic framework for registering and creating providers will be added to `internal/registry/compreg.go`.
2.  **Implement and Register Session Providers:** The existing "database" and "gnogent" session provider logic will be extracted into dedicated creator functions and registered with the new framework.
3.  **Implement and Register Memory Providers:** Similarly, the "database" and "gnogent" memory provider logic will be extracted and registered.
4.  **Update `BuildLauncherConfig`:** The `BuildLauncherConfig` function will be updated to use the new registry, removing the `switch/case` blocks.

## 3. Detailed Implementation Plan

### Step 3.1: Create Generic Provider Framework

In `internal/registry/compreg.go`, we will add a generic factory and registration functions for providers.

```go
// internal/registry/compreg.go

// ProviderCreator defines the function signature for creating a provider service.
type ProviderCreator[C any, S any] func(cfg *C) (S, error)

// RegisterProvider registers a new provider creator for a specific service type.
// serviceType should be "session", "memory", etc.
func RegisterProvider[C any, S any](serviceType, providerName string, creator ProviderCreator[C, S]) {
	key := fmt.Sprintf("%s:%s", serviceType, providerName)
	compreg.Set(key, creator)
}

// CreateProvider instantiates a provider service from the registry.
func CreateProvider[C any, S any](ctx context.Context, serviceType, providerName string, cfg *C) (S, error) {
	key := fmt.Sprintf("%s:%s", serviceType, providerName)
	
	var zero S
	creatorAny, ok := compreg.Lookup[any](key)
	if !ok {
		return zero, fmt.Errorf("%s provider %q not found", serviceType, providerName)
	}

	creator, ok := creatorAny.(ProviderCreator[C, S])
	if !ok {
		return zero, fmt.Errorf("invalid creator type for %s provider %q", serviceType, providerName)
	}

	return creator(cfg)
}
```

### Step 3.2: Implement and Register Session Providers

A new file, `internal/registry/session_providers.go`, will be created to house the session provider logic.

```go
// internal/registry/session_providers.go

package registry

import (
	"fmt"
	"github.com/innomon/agentic/internal/gnogent/storage"
	"google.golang.org/adk/session"
	sessiondb "google.golang.org/adk/session/database"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func databaseSessionCreator(cfg *SessionConfig) (session.Service, error) {
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

func gnogentSessionCreator(cfg *SessionConfig) (session.Service, error) {
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
```

### Step 3.3: Implement and Register Memory Providers

A new file, `internal/registry/memory_providers.go`, will be created for memory provider logic.

```go
// internal/registry/memory_providers.go

package registry

import (
	"fmt"
	"github.com/innomon/agentic/internal/gnogent/storage"
	"github.com/innomon/agentic/internal/memory"
	adkmemory "google.golang.org/adk/memory"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func databaseMemoryCreator(cfg *MemoryConfig) (adkmemory.Service, error) {
	return memory.OpenDatabaseMemoryService(cfg.Driver, cfg.DSN, cfg.AutoMigrate)
}

func gnogentMemoryCreator(cfg *MemoryConfig) (adkmemory.Service, error) {
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
```

### Step 3.4: Update `BuildLauncherConfig`

Finally, `internal/registry/launcher.go` will be refactored to use the new provider registry.

```go
// internal/registry/launcher.go

// ... (imports)

func (r *Registry) BuildLauncherConfig(ctx context.Context) (*launcher.Config, error) {
	rootAgent, err := r.GetRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create root agent: %w", err)
	}

	cfg := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(rootAgent),
		SessionService: session.InMemoryService(),
		MemoryService:  adkmemory.InMemoryService(),
	}

	if r.cfg.Session != nil {
		sessionSvc, err := CreateProvider[SessionConfig, session.Service](ctx, "session", r.cfg.Session.Provider, r.cfg.Session)
		if err != nil {
			return nil, fmt.Errorf("failed to create session service: %w", err)
		}
		cfg.SessionService = sessionSvc
	}

	if r.cfg.Memory != nil {
		memorySvc, err := CreateProvider[MemoryConfig, adkmemory.Service](ctx, "memory", r.cfg.Memory.Provider, r.cfg.Memory)
		if err != nil {
			return nil, fmt.Errorf("failed to create memory service: %w", err)
		}
		cfg.MemoryService = memorySvc
	}

	return cfg, nil
}

// ... The helper functions openDialector, openGnogentSessionService, 
// and openGnogentMemoryService can now be moved into their respective
// provider creator files or removed if the logic is simple enough to inline.
```

## 4. Conclusion

This refactoring will decouple the launcher from specific provider implementations, making it easier to add, remove, or modify providers in the future without changing core logic. It standardizes the provider creation process, bringing it in line with other components in the registry.
