package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/innomon/agentic/pkg/registry"
	"gopkg.in/yaml.v3"
)

type Config = registry.Config

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var raw registry.RawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	raw.BasePath = filepath.Dir(path)

	return registry.ParseRaw(&raw)
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
