package gomlx

import (
	"os"
	"testing"
)

func TestLoadWeights(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		if _, err := os.Stat("testdata/test.gguf"); err == nil {
			modelPath = "testdata/test.gguf"
		}
	}
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a .gguf file path to run weight loading tests")
	}

	weights, err := LoadWeights(modelPath)
	if err != nil {
		t.Fatalf("LoadWeights: %v", err)
	}
	if len(weights) == 0 {
		t.Fatal("expected non-empty weight map")
	}

	for name, w := range weights {
		if len(w.Data) == 0 {
			t.Errorf("tensor %q has empty data", name)
		}
		if len(w.Shape) == 0 {
			t.Errorf("tensor %q has empty shape", name)
		}
		t.Logf("Loaded tensor %s: shape=%v, len(data)=%d", name, w.Shape, len(w.Data))
		break
	}
}
