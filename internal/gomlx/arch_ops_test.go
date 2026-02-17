package gomlx

import (
	"math"
	"os"
	"testing"
)

func TestMatVecMul(t *testing.T) {
	// A = [[1, 2, 3],
	//      [4, 5, 6]]  (2x3)
	// x = [1, 1, 1]
	// y = [6, 15]
	a := []float32{1, 2, 3, 4, 5, 6}
	x := []float32{1, 1, 1}
	y := MatVecMul(a, x, 2, 3)

	if len(y) != 2 {
		t.Fatalf("expected len 2, got %d", len(y))
	}
	assertClose(t, "y[0]", y[0], 6)
	assertClose(t, "y[1]", y[1], 15)
}

func TestMatVecMulIdentity(t *testing.T) {
	// Identity matrix times a vector should return the vector.
	a := []float32{1, 0, 0, 0, 1, 0, 0, 0, 1}
	x := []float32{3, 7, 11}
	y := MatVecMul(a, x, 3, 3)

	assertClose(t, "y[0]", y[0], 3)
	assertClose(t, "y[1]", y[1], 7)
	assertClose(t, "y[2]", y[2], 11)
}

func TestMatMul(t *testing.T) {
	// A = [[1, 2], [3, 4]]  (2x2)
	// B = [[5, 6], [7, 8]]  (2x2)
	// C = [[19, 22], [43, 50]]
	a := []float32{1, 2, 3, 4}
	b := []float32{5, 6, 7, 8}
	c := MatMul(a, b, 2, 2, 2)

	if len(c) != 4 {
		t.Fatalf("expected len 4, got %d", len(c))
	}
	assertClose(t, "c[0]", c[0], 19)
	assertClose(t, "c[1]", c[1], 22)
	assertClose(t, "c[2]", c[2], 43)
	assertClose(t, "c[3]", c[3], 50)
}

func TestRMSNorm(t *testing.T) {
	x := []float32{1, 2, 3, 4}
	w := []float32{1, 1, 1, 1}
	out := make([]float32, 4)

	RMSNorm(out, x, w, 1e-5)

	// RMS = sqrt(mean(1+4+9+16)) = sqrt(30/4) = sqrt(7.5)
	rms := float32(math.Sqrt(7.5 + 1e-5))
	for i, xi := range x {
		expected := xi / rms
		assertClose(t, "out", out[i], expected)
	}
}

func TestRMSNormWithWeights(t *testing.T) {
	x := []float32{2, 4}
	w := []float32{0.5, 2.0}
	out := make([]float32, 2)

	RMSNorm(out, x, w, 1e-5)

	// RMS = sqrt((4+16)/2 + eps) = sqrt(10 + eps)
	rms := float32(math.Sqrt(10 + 1e-5))
	assertClose(t, "out[0]", out[0], 2*0.5/rms)
	assertClose(t, "out[1]", out[1], 4*2.0/rms)
}

func TestSoftmax(t *testing.T) {
	x := []float32{1, 2, 3, 4}
	Softmax(x)

	var sum float32
	for _, v := range x {
		sum += v
		if v < 0 {
			t.Errorf("softmax value %f < 0", v)
		}
	}
	assertClose(t, "sum", sum, 1.0)

	// Values should be monotonically increasing.
	for i := 1; i < len(x); i++ {
		if x[i] < x[i-1] {
			t.Errorf("softmax not monotonic: x[%d]=%f < x[%d]=%f", i, x[i], i-1, x[i-1])
		}
	}
}

func TestSoftmaxUniform(t *testing.T) {
	x := []float32{0, 0, 0}
	Softmax(x)

	for i, v := range x {
		assertCloseEps(t, "uniform", v, 1.0/3.0, 1e-6)
		_ = i
	}
}

func TestSiLU(t *testing.T) {
	x := []float32{0, 1, -1, 2}
	SiLU(x)

	// SiLU(0) = 0 * sigmoid(0) = 0
	assertCloseEps(t, "silu(0)", x[0], 0, 1e-6)

	// SiLU(1) = 1 * sigmoid(1) = 1/(1+exp(-1)) ≈ 0.7311
	assertCloseEps(t, "silu(1)", x[1], float32(1.0/(1.0+math.Exp(-1))), 1e-5)

	// SiLU(-1) = -1 * sigmoid(-1) = -1/(1+exp(1)) ≈ -0.2689
	assertCloseEps(t, "silu(-1)", x[2], float32(-1.0/(1.0+math.Exp(1))), 1e-5)

	// SiLU(2) = 2 * sigmoid(2) ≈ 1.7616
	assertCloseEps(t, "silu(2)", x[3], float32(2.0/(1.0+math.Exp(-2))), 1e-5)
}

func TestRoPE(t *testing.T) {
	headDim := 4
	nHeads := 1
	nKVHeads := 1

	// At pos=0, RoPE should be identity (cos=1, sin=0).
	q := []float32{1, 2, 3, 4}
	k := []float32{5, 6, 7, 8}
	qOrig := make([]float32, len(q))
	kOrig := make([]float32, len(k))
	copy(qOrig, q)
	copy(kOrig, k)

	RoPE(q, k, headDim, 0, nHeads, nKVHeads, 10000)

	// At pos=0, theta=0 for all dimensions, so cos=1, sin=0. Values unchanged.
	for i := range q {
		assertCloseEps(t, "q pos=0", q[i], qOrig[i], 1e-5)
	}
	for i := range k {
		assertCloseEps(t, "k pos=0", k[i], kOrig[i], 1e-5)
	}

	// At pos=1, values should change.
	q2 := []float32{1, 0, 1, 0}
	k2 := []float32{1, 0, 1, 0}
	RoPE(q2, k2, headDim, 1, nHeads, nKVHeads, 10000)

	// First pair rotated by theta = 1 * 1/10000^(0/4) = 1.0
	cos0 := float32(math.Cos(1.0))
	sin0 := float32(math.Sin(1.0))
	assertCloseEps(t, "q2[0]", q2[0], cos0, 1e-5)
	assertCloseEps(t, "q2[1]", q2[1], sin0, 1e-5)

	// Verify norm preservation: RoPE is a rotation so |v| should be preserved.
	origNorm := float32(math.Sqrt(float64(1*1 + 0*0)))
	newNorm := float32(math.Sqrt(float64(q2[0]*q2[0] + q2[1]*q2[1])))
	assertCloseEps(t, "norm preserved", newNorm, origNorm, 1e-5)
}

func TestAdd(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{4, 5, 6}
	out := make([]float32, 3)
	Add(out, a, b)

	assertClose(t, "out[0]", out[0], 5)
	assertClose(t, "out[1]", out[1], 7)
	assertClose(t, "out[2]", out[2], 9)
}

func TestElemMul(t *testing.T) {
	a := []float32{2, 3, 4}
	b := []float32{5, 6, 7}
	out := make([]float32, 3)
	ElemMul(out, a, b)

	assertClose(t, "out[0]", out[0], 10)
	assertClose(t, "out[1]", out[1], 18)
	assertClose(t, "out[2]", out[2], 28)
}

func TestKVCache(t *testing.T) {
	kv := NewKVCache(2, 2, 4, 10)

	if kv.CurrentSeqLen() != 0 {
		t.Fatalf("expected seq len 0, got %d", kv.CurrentSeqLen())
	}

	// Add one position to layer 0.
	keys := []float32{1, 2, 3, 4, 5, 6, 7, 8}   // 2 heads * 4 dim
	vals := []float32{9, 10, 11, 12, 13, 14, 15, 16}
	kv.Update(0, keys, vals)

	if kv.CurrentSeqLen() != 1 {
		t.Fatalf("expected seq len 1, got %d", kv.CurrentSeqLen())
	}

	k0 := kv.KeysForHead(0, 0)
	if len(k0) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(k0))
	}
	assertClose(t, "k0[0]", k0[0], 1)
	assertClose(t, "k0[3]", k0[3], 4)

	v1 := kv.ValuesForHead(0, 1)
	assertClose(t, "v1[0]", v1[0], 13)

	// Add another position.
	kv.Update(0, keys, vals)
	if kv.CurrentSeqLen() != 2 {
		t.Fatalf("expected seq len 2, got %d", kv.CurrentSeqLen())
	}

	// Reset.
	kv.Reset()
	if kv.CurrentSeqLen() != 0 {
		t.Fatalf("expected seq len 0 after reset, got %d", kv.CurrentSeqLen())
	}
}

func TestLlamaForward(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a LLaMA GGUF file path")
	}

	info, err := ParseGGUF(modelPath)
	if err != nil {
		t.Fatalf("ParseGGUF: %v", err)
	}

	weights, err := LoadWeights(modelPath)
	if err != nil {
		t.Fatalf("LoadWeights: %v", err)
	}

	arch, err := NewLlamaArch(weights, info)
	if err != nil {
		t.Fatalf("NewLlamaArch: %v", err)
	}

	maxSeq := 128
	if int(info.ContextLength) < maxSeq {
		maxSeq = int(info.ContextLength)
	}
	kv := arch.NewKVCacheForModel(maxSeq)

	// Run forward pass with a single token (BOS = 1 for most LLaMA models).
	logits := arch.Forward([]int32{1}, 0, kv)

	if len(logits) != arch.vocabSize {
		t.Fatalf("expected logits len %d, got %d", arch.vocabSize, len(logits))
	}

	// Sanity check: logits should contain finite values.
	for i, v := range logits {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			t.Fatalf("logit[%d] is %f", i, v)
		}
	}

	t.Logf("Forward pass OK: vocabSize=%d, logits range [%f, %f]",
		len(logits), minFloat32(logits), maxFloat32(logits))
}

// --- helpers ---

func assertClose(t *testing.T, name string, got, want float32) {
	t.Helper()
	assertCloseEps(t, name, got, want, 1e-4)
}

func assertCloseEps(t *testing.T, name string, got, want, eps float32) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > eps {
		t.Errorf("%s: got %f, want %f (diff %f > eps %f)", name, got, want, diff, eps)
	}
}

func minFloat32(s []float32) float32 {
	m := s[0]
	for _, v := range s[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func maxFloat32(s []float32) float32 {
	m := s[0]
	for _, v := range s[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
