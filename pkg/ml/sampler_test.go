package ml

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestSamplerArgmax(t *testing.T) {
	logits := []float32{1.0, 3.0, 2.0, 0.5}
	config := SamplerConfig{Temperature: 0}
	id := Sample(logits, config, nil, nil)
	if id != 1 {
		t.Errorf("expected token 1 (highest logit), got %d", id)
	}
}

func TestSamplerArgmaxNegative(t *testing.T) {
	logits := []float32{-5.0, -1.0, -3.0, -10.0}
	config := SamplerConfig{Temperature: 0}
	id := Sample(logits, config, nil, nil)
	if id != 1 {
		t.Errorf("expected token 1 (least negative logit), got %d", id)
	}
}

func TestSamplerTopK(t *testing.T) {
	// With topK=2, only the top 2 tokens (indices 1 and 2) should be sampled.
	logits := []float32{0.0, 10.0, 9.0, 0.0, 0.0}
	config := SamplerConfig{Temperature: 1.0, TopK: 2, TopP: 1.0}
	rng := rand.New(rand.NewPCG(42, 0))

	counts := make(map[int32]int)
	for i := 0; i < 1000; i++ {
		id := Sample(logits, config, rng, nil)
		counts[id]++
	}

	// Tokens 0, 3, 4 should never be sampled with topK=2.
	for _, forbidden := range []int32{0, 3, 4} {
		if counts[forbidden] > 0 {
			t.Errorf("token %d was sampled %d times with topK=2", forbidden, counts[forbidden])
		}
	}
	// Tokens 1 and 2 should both be sampled.
	if counts[1] == 0 {
		t.Error("token 1 was never sampled")
	}
	if counts[2] == 0 {
		t.Error("token 2 was never sampled")
	}
}

func TestSamplerTopP(t *testing.T) {
	// Create logits where one token dominates.
	// With topP=0.5, only the dominant token should be sampled.
	logits := []float32{0.0, 100.0, 0.0, 0.0}
	config := SamplerConfig{Temperature: 1.0, TopK: 0, TopP: 0.5}
	rng := rand.New(rand.NewPCG(42, 0))

	for i := 0; i < 100; i++ {
		id := Sample(logits, config, rng, nil)
		if id != 1 {
			t.Errorf("expected token 1 (dominant), got %d", id)
		}
	}
}

func TestSamplerRepetitionPenalty(t *testing.T) {
	// Token 0 has the highest logit. With repetition penalty on token 0,
	// its logit should be reduced, making token 1 more likely.
	logits := []float32{5.0, 4.9, 0.0, 0.0}
	config := SamplerConfig{Temperature: 0, RepetitionPenalty: 2.0}
	recentTokens := []int32{0}

	id := Sample(logits, config, nil, recentTokens)
	// Token 0's logit (5.0) becomes 5.0/2.0 = 2.5, so token 1 (4.9) wins.
	if id != 1 {
		t.Errorf("expected token 1 after repetition penalty, got %d", id)
	}
}

func TestSamplerRepetitionPenaltyNegative(t *testing.T) {
	// For negative logits, repetition penalty multiplies (making them more negative).
	logits := []float32{-1.0, -1.5, -2.0}
	config := SamplerConfig{Temperature: 0, RepetitionPenalty: 2.0}

	// Without penalty, token 0 wins (least negative).
	id := Sample(logits, config, nil, nil)
	if id != 0 {
		t.Errorf("expected token 0 without penalty, got %d", id)
	}

	// With penalty on token 0: -1.0 * 2.0 = -2.0, so token 1 (-1.5) wins.
	recentTokens := []int32{0}
	id = Sample(logits, config, nil, recentTokens)
	if id != 1 {
		t.Errorf("expected token 1 after repetition penalty, got %d", id)
	}
}

func TestSamplerDeterministicWithSeed(t *testing.T) {
	logits := []float32{1.0, 1.0, 1.0, 1.0}
	config := SamplerConfig{Temperature: 1.0, TopK: 0, TopP: 1.0}

	// Same seed should produce same sequence.
	rng1 := rand.New(rand.NewPCG(123, 456))
	rng2 := rand.New(rand.NewPCG(123, 456))

	for i := 0; i < 100; i++ {
		id1 := Sample(logits, config, rng1, nil)
		id2 := Sample(logits, config, rng2, nil)
		if id1 != id2 {
			t.Fatalf("iteration %d: different results with same seed: %d vs %d", i, id1, id2)
		}
	}
}

func TestSamplerEmptyLogits(t *testing.T) {
	id := Sample([]float32{}, SamplerConfig{}, nil, nil)
	if id != 0 {
		t.Errorf("expected 0 for empty logits, got %d", id)
	}
}

func TestSamplerTemperatureEffect(t *testing.T) {
	// High temperature should produce more uniform distribution.
	// Low temperature should concentrate on the top token.
	logits := []float32{2.0, 1.0, 0.5, 0.0}
	rng := rand.New(rand.NewPCG(42, 0))
	n := 5000

	// Low temperature: token 0 should dominate.
	lowTemp := SamplerConfig{Temperature: 0.1, TopK: 0, TopP: 1.0}
	lowCounts := make(map[int32]int)
	for i := 0; i < n; i++ {
		lowCounts[Sample(logits, lowTemp, rng, nil)]++
	}

	// High temperature: more spread out.
	highTemp := SamplerConfig{Temperature: 5.0, TopK: 0, TopP: 1.0}
	highCounts := make(map[int32]int)
	for i := 0; i < n; i++ {
		highCounts[Sample(logits, highTemp, rng, nil)]++
	}

	// With low temp, token 0 should get the vast majority.
	lowRatio := float64(lowCounts[0]) / float64(n)
	highRatio := float64(highCounts[0]) / float64(n)

	if lowRatio < 0.8 {
		t.Errorf("low temperature: expected token 0 ratio > 0.8, got %.3f", lowRatio)
	}
	if highRatio > 0.5 {
		t.Errorf("high temperature: expected token 0 ratio < 0.5, got %.3f", highRatio)
	}
}

func TestSamplerDoesNotMutateInput(t *testing.T) {
	logits := []float32{1.0, 3.0, 2.0, 0.5}
	original := make([]float32, len(logits))
	copy(original, logits)

	config := SamplerConfig{Temperature: 0.5, TopK: 2, RepetitionPenalty: 1.5}
	rng := rand.New(rand.NewPCG(42, 0))
	Sample(logits, config, rng, []int32{1})

	for i := range logits {
		if math.Abs(float64(logits[i]-original[i])) > 1e-9 {
			t.Errorf("logits[%d] was mutated: %f → %f", i, original[i], logits[i])
		}
	}
}
