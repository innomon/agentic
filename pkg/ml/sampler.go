package ml

import (
	"math"
	"math/rand/v2"
	"slices"
)

// SamplerConfig controls how the next token is selected from logits.
type SamplerConfig struct {
	Temperature       float32
	TopK              int
	TopP              float32
	RepetitionPenalty float32
}

// DefaultSamplerConfig returns a reasonable default sampling configuration.
func DefaultSamplerConfig() SamplerConfig {
	return SamplerConfig{
		Temperature: 1.0,
		TopK:        40,
		TopP:        0.9,
	}
}

// Sample selects the next token ID from logits using the configured sampling strategy.
// If temperature is 0 or very small, greedy argmax is used.
// recentTokens is used for repetition penalty; rng is used for random sampling.
func Sample(logits []float32, config SamplerConfig, rng *rand.Rand, recentTokens []int32) int32 {
	n := len(logits)
	if n == 0 {
		return 0
	}

	// Work on a copy to avoid mutating the caller's slice.
	work := make([]float32, n)
	copy(work, logits)

	// 1. Repetition penalty.
	if config.RepetitionPenalty > 0 && config.RepetitionPenalty != 1.0 && len(recentTokens) > 0 {
		applyRepetitionPenalty(work, recentTokens, config.RepetitionPenalty)
	}

	// 2. Temperature 0 (or very small) → greedy argmax.
	if config.Temperature < 1e-6 {
		return argmax(work)
	}

	// 3. Temperature scaling.
	if config.Temperature != 1.0 {
		invTemp := 1.0 / config.Temperature
		for i := range work {
			work[i] *= invTemp
		}
	}

	// Build (index, logit) pairs for sorting.
	type indexedLogit struct {
		idx   int32
		logit float32
	}
	items := make([]indexedLogit, n)
	for i := range work {
		items[i] = indexedLogit{int32(i), work[i]}
	}
	// Sort descending by logit.
	slices.SortFunc(items, func(a, b indexedLogit) int {
		if a.logit > b.logit {
			return -1
		}
		if a.logit < b.logit {
			return 1
		}
		return 0
	})

	// 4. Top-K filtering.
	k := n
	if config.TopK > 0 && config.TopK < n {
		k = config.TopK
	}
	items = items[:k]

	// 5. Softmax over kept items.
	maxVal := items[0].logit
	probs := make([]float32, len(items))
	var sum float32
	for i, it := range items {
		probs[i] = float32(math.Exp(float64(it.logit - maxVal)))
		sum += probs[i]
	}
	inv := 1.0 / sum
	for i := range probs {
		probs[i] *= inv
	}

	// 6. Top-P (nucleus) filtering.
	if config.TopP > 0 && config.TopP < 1.0 {
		var cumProb float32
		cutoff := len(probs)
		for i, p := range probs {
			cumProb += p
			if cumProb >= config.TopP {
				cutoff = i + 1
				break
			}
		}
		items = items[:cutoff]
		probs = probs[:cutoff]

		// Renormalize.
		sum = 0
		for _, p := range probs {
			sum += p
		}
		inv = 1.0 / sum
		for i := range probs {
			probs[i] *= inv
		}
	}

	// 7. Random sampling from the distribution.
	r := rng.Float32()
	var cumulative float32
	for i, p := range probs {
		cumulative += p
		if r < cumulative {
			return items[i].idx
		}
	}
	// Fallback to last item (rounding).
	return items[len(items)-1].idx
}

// applyRepetitionPenalty reduces the logit of recently generated tokens.
// If the logit is positive, it is divided by the penalty; if negative, it is multiplied.
func applyRepetitionPenalty(logits []float32, recentTokens []int32, penalty float32) {
	for _, tok := range recentTokens {
		idx := int(tok)
		if idx < 0 || idx >= len(logits) {
			continue
		}
		if logits[idx] > 0 {
			logits[idx] /= penalty
		} else {
			logits[idx] *= penalty
		}
	}
}

// argmax returns the index of the maximum value in logits.
func argmax(logits []float32) int32 {
	best := int32(0)
	bestVal := logits[0]
	for i := 1; i < len(logits); i++ {
		if logits[i] > bestVal {
			bestVal = logits[i]
			best = int32(i)
		}
	}
	return best
}
