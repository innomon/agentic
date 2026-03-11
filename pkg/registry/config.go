package registry

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type RawConfig struct {
	Models    map[string]*yaml.Node
	Agents    map[string]*yaml.Node
	Tools     map[string]*yaml.Node
	Sandboxes map[string]*yaml.Node
	Session   *yaml.Node
	Memory    *yaml.Node
	Auth      *yaml.Node
	RootAgent string
}

func (r *RawConfig) UnmarshalYAML(node *yaml.Node) error {
	r.Models = make(map[string]*yaml.Node)
	r.Agents = make(map[string]*yaml.Node)
	r.Tools = make(map[string]*yaml.Node)
	r.Sandboxes = make(map[string]*yaml.Node)
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node")
	}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		switch key {
		case "models":
			if val.Kind == yaml.MappingNode {
				for j := 0; j < len(val.Content); j += 2 {
					r.Models[val.Content[j].Value] = val.Content[j+1]
				}
			}
		case "agents":
			if val.Kind == yaml.MappingNode {
				for j := 0; j < len(val.Content); j += 2 {
					r.Agents[val.Content[j].Value] = val.Content[j+1]
				}
			}
		case "tools":
			if val.Kind == yaml.MappingNode {
				for j := 0; j < len(val.Content); j += 2 {
					r.Tools[val.Content[j].Value] = val.Content[j+1]
				}
			}
		case "sandboxes":
			if val.Kind == yaml.MappingNode {
				for j := 0; j < len(val.Content); j += 2 {
					r.Sandboxes[val.Content[j].Value] = val.Content[j+1]
				}
			}
		case "session":
			r.Session = val
		case "memory":
			r.Memory = val
		case "auth":
			r.Auth = val
		case "root_agent":
			r.RootAgent = val.Value
		}
	}
	return nil
}

type ModelEntry struct {
	Name     string
	Provider string
	Config   any
}

type AgentEntry struct {
	Name      string
	Type      string
	SubAgents []string
	Tools     []string
	Config    any
}

type ToolEntry struct {
	Name   string
	Type   string
	Config any
}

type SandboxEntry struct {
	Name   string
	Type   string
	Config any
}

type SessionConfig struct {
	Provider        string `yaml:"provider"`
	Driver          string `yaml:"driver"`
	DSN             string `yaml:"dsn"`
	AutoMigrate     bool   `yaml:"auto_migrate"`
	Project         string `yaml:"project"`
	Location        string `yaml:"location"`
	ReasoningEngine string `yaml:"reasoning_engine"`
}

type MemoryConfig struct {
	Provider    string `yaml:"provider"`
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	AutoMigrate bool   `yaml:"auto_migrate"`
	KBPath      string `yaml:"kb_path"`
}

type GnogentServiceConfig struct {
	DSN         string `yaml:"dsn"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type AuthConfig struct {
	JWT *JWTConfig `yaml:"jwt"`
}

type JWTConfig struct {
	PublicKeyPath string `yaml:"public_key_path"`
	Issuer        string `yaml:"issuer"`
	Audience      string `yaml:"audience"`
}

type Config struct {
	Models    map[string]ModelEntry
	Agents    map[string]AgentEntry
	Tools     map[string]ToolEntry
	Sandboxes map[string]SandboxEntry
	Session   *SessionConfig
	Memory    *MemoryConfig
	Auth      *AuthConfig
	RootAgent string
}

func ParseRaw(raw *RawConfig) (*Config, error) {
	cfg := &Config{
		Models:    make(map[string]ModelEntry),
		Agents:    make(map[string]AgentEntry),
		Tools:     make(map[string]ToolEntry),
		Sandboxes: make(map[string]SandboxEntry),
		RootAgent: raw.RootAgent,
	}

	for name, node := range raw.Models {
		provider, cfgAny, err := DecodeModelConfig(name, node)
		if err != nil {
			return nil, err
		}
		cfg.Models[name] = ModelEntry{
			Name:     name,
			Provider: provider,
			Config:   cfgAny,
		}
	}

	for name, node := range raw.Agents {
		typeName, cfgAny, err := DecodeAgentConfig(name, node)
		if err != nil {
			return nil, err
		}

		var subAgents []string
		var tools []string
		if base, ok := cfgAny.(interface{ GetSubAgents() []string }); ok {
			subAgents = base.GetSubAgents()
		} else {
			var d struct {
				SubAgents []string `yaml:"sub_agents"`
			}
			_ = node.Decode(&d)
			subAgents = d.SubAgents
		}

		var td struct {
			Tools []string `yaml:"tools"`
		}
		_ = node.Decode(&td)
		tools = td.Tools

		cfg.Agents[name] = AgentEntry{
			Name:      name,
			Type:      typeName,
			SubAgents: subAgents,
			Tools:     tools,
			Config:    cfgAny,
		}
	}

	for name, node := range raw.Tools {
		typeName, cfgAny, err := DecodeToolConfig(name, node)
		if err != nil {
			return nil, err
		}
		cfg.Tools[name] = ToolEntry{
			Name:   name,
			Type:   typeName,
			Config: cfgAny,
		}
	}

	for name, node := range raw.Sandboxes {
		typeName, cfgAny, err := DecodeSandboxConfig(name, node)
		if err != nil {
			return nil, err
		}
		cfg.Sandboxes[name] = SandboxEntry{
			Name:   name,
			Type:   typeName,
			Config: cfgAny,
		}
	}

	if raw.Session != nil {
		var sess SessionConfig
		if err := raw.Session.Decode(&sess); err != nil {
			return nil, fmt.Errorf("failed to parse session config: %w", err)
		}
		cfg.Session = &sess
	}

	if raw.Memory != nil {
		var mem MemoryConfig
		if err := raw.Memory.Decode(&mem); err != nil {
			return nil, fmt.Errorf("failed to parse memory config: %w", err)
		}
		cfg.Memory = &mem
	}

	if raw.Auth != nil {
		var auth AuthConfig
		if err := raw.Auth.Decode(&auth); err != nil {
			return nil, fmt.Errorf("failed to parse auth config: %w", err)
		}
		cfg.Auth = &auth
	}

	return cfg, nil
}

func (c *Config) GetDefaultModel() (string, ModelEntry, error) {
	for name, m := range c.Models {
		if d, ok := m.Config.(interface{ IsDefault() bool }); ok && d.IsDefault() {
			return name, m, nil
		}
	}

	for name, m := range c.Models {
		return name, m, nil
	}

	return "", ModelEntry{}, fmt.Errorf("no models configured")
}

func (c *Config) GetAgent(name string) (*AgentEntry, error) {
	agent, ok := c.Agents[name]
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}
	return &agent, nil
}

func (c *Config) GetModel(name string) (ModelEntry, error) {
	model, ok := c.Models[name]
	if !ok {
		return ModelEntry{}, fmt.Errorf("model %q not found", name)
	}
	return model, nil
}

func (c *Config) GetTool(name string) (*ToolEntry, error) {
	tool, ok := c.Tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return &tool, nil
}
