package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Models map[string]ModelConfig `yaml:"models"`
	Agents map[string]AgentConfig `yaml:"agents"`
}

type ModelConfig struct {
	Provider string `yaml:"provider"`
	ModelID  string `yaml:"model_id"`
	Default  bool   `yaml:"default"`
}

type AgentConfig struct {
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	SubAgents   []string `yaml:"sub_agents"`
	Instruction string   `yaml:"instruction"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
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

func (c *Config) GetDefaultModel() (string, ModelConfig, error) {
	for name, m := range c.Models {
		if m.Default {
			return name, m, nil
		}
	}

	for name, m := range c.Models {
		return name, m, nil
	}

	return "", ModelConfig{}, fmt.Errorf("no models configured")
}

func (c *Config) GetAgent(name string) (AgentConfig, error) {
	agent, ok := c.Agents[name]
	if !ok {
		return AgentConfig{}, fmt.Errorf("agent %q not found", name)
	}
	return agent, nil
}

func (c *Config) GetModel(name string) (ModelConfig, error) {
	model, ok := c.Models[name]
	if !ok {
		return ModelConfig{}, fmt.Errorf("model %q not found", name)
	}
	return model, nil
}
