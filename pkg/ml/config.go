package ml

import (
	"fmt"

	"github.com/innomon/agentic/pkg/registry"
)

type MLConfig struct {
	registry.ModelBase `yaml:",inline"`
	ModelPath          string `yaml:"model_path"`
	Backend            string `yaml:"backend"`
	BackendConfig      string `yaml:"backend_config"`
	ContextLength      int    `yaml:"context_length"`
	Threads            int    `yaml:"threads"`
	TokenizerPath      string `yaml:"tokenizer_path"`
	MemoryBudgetMB     int    `yaml:"memory_budget_mb"`
}

func (c *MLConfig) Validate() error {
	if err := c.ModelBase.Validate(); err != nil {
		return err
	}
	if c.ModelPath == "" {
		return fmt.Errorf("model_path is required for ml provider")
	}
	return nil
}
