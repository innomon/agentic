package routing

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/innomon/agentic/pkg/auth"
	"github.com/innomon/agentic/pkg/registry"
	"github.com/innomon/agentic/pkg/userdb"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"gorm.io/gorm"
)

type DBConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type UserDBToolConfig struct {
	registry.ToolBase `yaml:",inline"`
	Op                string   `yaml:"op"`
	DB                DBConfig `yaml:"db"`
	AdminUsers        []string `yaml:"admin_users"`
}

func (c *UserDBToolConfig) Validate() error {
	if c.Op == "" {
		return fmt.Errorf("op is required for userdb tool")
	}
	switch c.Op {
	case "get_profile", "create_user", "update_status", "update_roles", "update_channels", "delete_user":
	default:
		return fmt.Errorf("unknown userdb op %q", c.Op)
	}
	if c.DB.Driver == "" {
		return fmt.Errorf("db.driver is required for userdb tool")
	}
	if c.DB.DSN == "" {
		return fmt.Errorf("db.dsn is required for userdb tool")
	}
	return nil
}

var (
	instancesMu sync.Mutex
	instances   = map[string]*userdb.UserDB{}
)

func getOrOpenUserDB(cfg DBConfig) (*userdb.UserDB, error) {
	instancesMu.Lock()
	defer instancesMu.Unlock()

	if db, ok := instances[cfg.DSN]; ok {
		return db, nil
	}

	db, err := userdb.Open(cfg.Driver, cfg.DSN, cfg.AutoMigrate)
	if err != nil {
		return nil, err
	}
	instances[cfg.DSN] = db
	return db, nil
}

func callerID(ctx context.Context) string {
	if claims := auth.ClaimsFromContext(ctx); claims != nil {
		return claims.UserID
	}
	return "system"
}

func toStringSlice(v any) ([]string, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", v)
	}
	out := make([]string, len(arr))
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("expected string at index %d, got %T", i, item)
		}
		out[i] = s
	}
	return out, nil
}

func userdbToolCreator(_ context.Context, name string, cfg *UserDBToolConfig, _ registry.SandboxRegistry) (tool.Tool, error) {
	db, err := getOrOpenUserDB(cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("userdb tool %q: %w", name, err)
	}
	db.SetAdminUsers(cfg.AdminUsers)

	var handler func(ctx agent.Context, args map[string]any) (any, error)

	switch cfg.Op {
	case "get_profile":
		handler = func(ctx agent.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}

			rec, err := db.GetUser(ctx, userID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return map[string]any{"found": false, "user_id": userID}, nil
				}
				return nil, err
			}

			roles := make([]string, len(rec.Profile.Roles))
			copy(roles, rec.Profile.Roles)
			isAdmin := db.IsAdmin(userID)
			if isAdmin {
				roles = append(roles, "admin")
			}

			return map[string]any{
				"user_id":  rec.UserID,
				"status":   string(rec.Status),
				"roles":    roles,
				"channels": rec.Profile.Channels,
				"is_admin": isAdmin,
			}, nil
		}

	case "create_user":
		handler = func(ctx agent.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}
			status, _ := args["status"].(string)
			if status == "" {
				status = string(userdb.StatusPending)
			}

			var channels []string
			if v, ok := args["channels"]; ok {
				var cerr error
				channels, cerr = toStringSlice(v)
				if cerr != nil {
					return nil, fmt.Errorf("channels: %w", cerr)
				}
			}

			var roles []string
			if v, ok := args["roles"]; ok {
				var rerr error
				roles, rerr = toStringSlice(v)
				if rerr != nil {
					return nil, fmt.Errorf("roles: %w", rerr)
				}
			}

			rec := &userdb.UserRecord{
				UserID: userID,
				Status: userdb.UserStatus(status),
				Profile: userdb.ProfileJSON{
					UserProfile: userdb.UserProfile{
						UserID:   userID,
						Channels: channels,
						Roles:    roles,
					},
				},
			}
			if err := db.CreateUser(ctx, rec); err != nil {
				return nil, err
			}
			return map[string]any{"created": true, "user_id": userID}, nil
		}

	case "update_status":
		handler = func(ctx agent.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}
			status, _ := args["status"].(string)
			if status == "" {
				return nil, fmt.Errorf("status is required")
			}
			if err := db.SetStatus(ctx, userID, userdb.UserStatus(status), callerID(ctx)); err != nil {
				return nil, err
			}
			return map[string]any{"updated": true, "user_id": userID, "status": status}, nil
		}

	case "update_roles":
		handler = func(ctx agent.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}
			rolesRaw, ok := args["roles"]
			if !ok {
				return nil, fmt.Errorf("roles is required")
			}
			roles, err := toStringSlice(rolesRaw)
			if err != nil {
				return nil, fmt.Errorf("roles: %w", err)
			}
			if err := db.SetRoles(ctx, userID, roles, callerID(ctx)); err != nil {
				return nil, err
			}
			return map[string]any{"updated": true, "user_id": userID, "roles": roles}, nil
		}

	case "update_channels":
		handler = func(ctx agent.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}
			channelsRaw, ok := args["channels"]
			if !ok {
				return nil, fmt.Errorf("channels is required")
			}
			channels, err := toStringSlice(channelsRaw)
			if err != nil {
				return nil, fmt.Errorf("channels: %w", err)
			}
			if err := db.SetChannels(ctx, userID, channels, callerID(ctx)); err != nil {
				return nil, err
			}
			return map[string]any{"updated": true, "user_id": userID, "channels": channels}, nil
		}

	case "delete_user":
		handler = func(ctx agent.Context, args map[string]any) (any, error) {
			userID, _ := args["user_id"].(string)
			if userID == "" {
				return nil, fmt.Errorf("user_id is required")
			}
			if err := db.DeleteUser(ctx, userID); err != nil {
				return nil, err
			}
			return map[string]any{"deleted": true, "user_id": userID}, nil
		}

	default:
		return nil, fmt.Errorf("unknown op %q", cfg.Op)
	}

	return functiontool.New(functiontool.Config{
		Name:        name,
		Description: cfg.Description,
	}, handler)
}

func init() {
	registry.RegisterToolType("userdb", userdbToolCreator)
}
