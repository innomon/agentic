package gomlx

import (
	"context"
	"os"
	"testing"
)

func TestGenerate(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a GGUF file path")
	}

	cfg := &GoMLXConfig{}
	cfg.ModelPath = modelPath
	cfg.ModelID = "test-model"

	m, err := NewGoMLXModel(cfg)
	if err != nil {
		t.Fatalf("NewGoMLXModel: %v", err)
	}

	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if m.arch == nil {
		t.Fatal("arch is nil after Init")
	}
	if m.tokenizer == nil {
		t.Fatal("tokenizer is nil after Init")
	}

	params := GenerateParams{
		MaxTokens: 20,
		Sampler:   DefaultSamplerConfig(),
	}

	ctx := context.Background()
	var output string
	for chunk, err := range Generate(ctx, m.arch, m.tokenizer, "Hello", params) {
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		output += chunk
	}

	if len(output) == 0 {
		t.Error("Generate produced no output")
	}
	t.Logf("Generated: %q", output)
}

func TestGenerateStopSequence(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a GGUF file path")
	}

	cfg := &GoMLXConfig{}
	cfg.ModelPath = modelPath
	cfg.ModelID = "test-model"

	m, err := NewGoMLXModel(cfg)
	if err != nil {
		t.Fatalf("NewGoMLXModel: %v", err)
	}
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	params := GenerateParams{
		MaxTokens:     100,
		StopSequences: []string{"\n"},
		Sampler:       DefaultSamplerConfig(),
	}

	ctx := context.Background()
	var output string
	for chunk, err := range Generate(ctx, m.arch, m.tokenizer, "Once upon a time", params) {
		if err != nil {
			t.Fatalf("Generate error: %v", err)
		}
		output += chunk
	}

	t.Logf("Generated (stop on newline): %q", output)
}

func TestGenerateContextCancel(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a GGUF file path")
	}

	cfg := &GoMLXConfig{}
	cfg.ModelPath = modelPath
	cfg.ModelID = "test-model"

	m, err := NewGoMLXModel(cfg)
	if err != nil {
		t.Fatalf("NewGoMLXModel: %v", err)
	}
	if err := m.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	params := GenerateParams{
		MaxTokens: 100,
		Sampler:   DefaultSamplerConfig(),
	}

	var gotErr bool
	for _, err := range Generate(ctx, m.arch, m.tokenizer, "Hello", params) {
		if err != nil {
			gotErr = true
			break
		}
	}

	if !gotErr {
		t.Error("expected error from cancelled context")
	}
}
