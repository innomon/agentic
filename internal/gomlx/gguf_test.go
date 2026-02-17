package gomlx

import (
	"os"
	"testing"
)

func TestGGUFParse(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		if _, err := os.Stat("testdata/test.gguf"); err == nil {
			modelPath = "testdata/test.gguf"
		}
	}
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a .gguf file path to run this test")
	}

	info, err := ParseGGUF(modelPath)
	if err != nil {
		t.Fatalf("ParseGGUF failed: %v", err)
	}

	if info.Architecture == "" {
		t.Error("expected non-empty architecture")
	}
	if info.TensorCount == 0 {
		t.Error("expected non-zero tensor count")
	}
	if info.ModelSize == 0 {
		t.Error("expected non-zero model size")
	}
	if info.FileSize == 0 {
		t.Error("expected non-zero file size")
	}
	if info.BitsPerWeight <= 0 {
		t.Error("expected positive bits per weight")
	}
	if info.EmbeddingLength == 0 {
		t.Error("expected non-zero embedding length")
	}
	if info.BlockCount == 0 {
		t.Error("expected non-zero block count")
	}
	if info.VocabSize == 0 {
		t.Error("expected non-zero vocab size")
	}
	if len(info.Tensors) == 0 {
		t.Error("expected non-empty tensor list")
	}

	mem := info.EstimateMemoryBytes()
	if mem <= info.ModelSize {
		t.Errorf("memory estimate %d should exceed model size %d", mem, info.ModelSize)
	}

	t.Logf("Architecture: %s", info.Architecture)
	t.Logf("ModelName: %s", info.ModelName)
	t.Logf("FileType: %s (%s)", info.FileType, info.FileTypeDescriptor)
	t.Logf("Parameters: %d", info.ModelParameters)
	t.Logf("Tensors: %d", info.TensorCount)
	t.Logf("EmbeddingLength: %d", info.EmbeddingLength)
	t.Logf("BlockCount: %d", info.BlockCount)
	t.Logf("ContextLength: %d", info.ContextLength)
	t.Logf("VocabSize: %d", info.VocabSize)
	t.Logf("ModelSize: %d bytes", info.ModelSize)
	t.Logf("EstimatedMemory: %d bytes", mem)
}
