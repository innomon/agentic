package server

// Config holds gateway server configuration.
type Config struct {
	Bind           string   `yaml:"bind"`             // Listen address (default "127.0.0.1:18789")
	Path           string   `yaml:"path"`             // WebSocket path (default "/ws")
	TickIntervalMs int      `yaml:"tick_interval_ms"` // Tick interval in ms (default 30000)
	MaxPayload     int64    `yaml:"max_payload"`      // Max message size in bytes (default 25*1024*1024)
	Tokens         []string `yaml:"tokens"`           // Allowed auth tokens
	Password       string   `yaml:"password"`         // Optional password
	AllowPassword  bool     `yaml:"allow_password"`   // Whether password auth is enabled
	RequireDevice  bool     `yaml:"require_device"`   // Require device signature
}

// SetDefaults fills in defaults for zero-value fields.
func (c *Config) SetDefaults() {
	if c.Bind == "" {
		c.Bind = "127.0.0.1:18789"
	}
	if c.Path == "" {
		c.Path = "/ws"
	}
	if c.TickIntervalMs == 0 {
		c.TickIntervalMs = 30000
	}
	if c.MaxPayload == 0 {
		c.MaxPayload = 25 * 1024 * 1024
	}
}
