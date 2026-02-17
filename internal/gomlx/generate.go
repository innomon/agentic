package gomlx

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"strings"
)

// GenerateParams controls autoregressive text generation.
type GenerateParams struct {
	MaxTokens     int
	StopSequences []string
	Sampler       SamplerConfig
}

// GenerateResult holds the result of text generation.
type GenerateResult struct {
	Text            string
	TokensGenerated int
	FinishReason    string // "stop", "max_tokens", "eos"
}

// Generate runs autoregressive text generation and yields text chunks as they are produced.
// The arch and tokenizer must be provided.
func Generate(ctx context.Context, arch Arch, tokenizer Tokenizer, prompt string, params GenerateParams) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		// 1. Tokenize the prompt.
		promptTokens, err := tokenizer.Encode(prompt)
		if err != nil {
			yield("", fmt.Errorf("tokenize prompt: %w", err))
			return
		}
		if len(promptTokens) == 0 {
			yield("", fmt.Errorf("prompt produced no tokens"))
			return
		}

		maxTokens := params.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 512
		}

		// 2. Create KV cache.
		maxSeqLen := len(promptTokens) + maxTokens
		kv := arch.NewCacheForModel(maxSeqLen)

		// 3. Prefill: run forward pass over all prompt tokens.
		// Forward processes them sequentially and populates the KV cache.
		// We only need the logits from the last token (for the first decode step).
		logits := arch.Forward(promptTokens, 0, kv)

		// 4. Decode loop.
		rng := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		eosID := tokenizer.EOSToken()

		var recentTokens []int32
		var genText strings.Builder

		for i := 0; i < maxTokens; i++ {
			// Check context cancellation.
			select {
			case <-ctx.Done():
				yield("", ctx.Err())
				return
			default:
			}

			// Sample next token from the logits.
			nextToken := Sample(logits, params.Sampler, rng, recentTokens)

			// Check EOS.
			if nextToken == eosID {
				return
			}

			// Decode the token to text.
			chunk, err := tokenizer.Decode([]int32{nextToken})
			if err != nil {
				yield("", fmt.Errorf("decode token %d: %w", nextToken, err))
				return
			}

			genText.WriteString(chunk)

			// Yield the text chunk. Check stop sequences after yielding is
			// intentional — we yield the chunk that completed the stop sequence.
			if !yield(chunk, nil) {
				return
			}

			// Check stop sequences.
			if matchesStopSequence(genText.String(), params.StopSequences) {
				return
			}

			// Bookkeeping.
			recentTokens = append(recentTokens, nextToken)
			if len(recentTokens) > 64 {
				recentTokens = recentTokens[len(recentTokens)-64:]
			}

			// Run forward pass for the newly sampled token to get next logits.
			// Position is len(promptTokens) + number of generated tokens so far.
			pos := len(promptTokens) + i
			logits = arch.Forward([]int32{nextToken}, pos, kv)
		}
	}
}

// matchesStopSequence checks if any stop sequence appears at the end of the generated text.
func matchesStopSequence(text string, stopSequences []string) bool {
	for _, seq := range stopSequences {
		if strings.HasSuffix(text, seq) {
			return true
		}
	}
	return false
}
