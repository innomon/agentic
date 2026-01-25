package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/innomon/med-agent/internal/componentreg"
	"gopkg.in/yaml.v3"
)

type RawConfig struct {
	Models map[string]*yaml.Node `yaml:"models"`
	Agents map[string]*yaml.Node `yaml:"agents"`
	Tools  map[string]*yaml.Node `yaml:"tools"`
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

type Config struct {
	Models map[string]ModelEntry
	Agents map[string]AgentEntry
	Tools  map[string]ToolEntry
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var raw RawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return parseAndValidate(&raw)
}

func parseAndValidate(raw *RawConfig) (*Config, error) {
	cfg := &Config{
		Models: make(map[string]ModelEntry),
		Agents: make(map[string]AgentEntry),
		Tools:  make(map[string]ToolEntry),
	}

	for name, node := range raw.Models {
		provider, cfgAny, err := componentreg.DecodeModelConfig(name, node)
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
		typeName, cfgAny, err := componentreg.DecodeAgentConfig(name, node)
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

		// Extract tools list
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

	// Parse tools
	for name, node := range raw.Tools {
		typeName, cfgAny, err := componentreg.DecodeToolConfig(name, node)
		if err != nil {
			return nil, err
		}
		cfg.Tools[name] = ToolEntry{
			Name:   name,
			Type:   typeName,
			Config: cfgAny,
		}
	}

	return cfg, nil
}

func LoadDefault() (*Config, error) {
	paths := []string{
		"config/config.yaml",
		"config.yaml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return Load(p)
		}
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		exeConfig := filepath.Join(exeDir, "config", "config.yaml")
		if _, err := os.Stat(exeConfig); err == nil {
			return Load(exeConfig)
		}
	}

	return nil, fmt.Errorf("config file not found in standard locations")
}

func (c *Config) GetDefaultModel() (string, ModelEntry, error) {
	for name, m := range c.Models {
		if base, ok := m.Config.(*componentreg.GeminiConfig); ok && base.Default {
			return name, m, nil
		}
		if base, ok := m.Config.(*componentreg.OpenAIConfig); ok && base.Default {
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
