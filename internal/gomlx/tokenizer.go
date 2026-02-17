package gomlx

import (
	"fmt"
	"strings"

	gguf "github.com/gpustack/gguf-parser-go"
)

// Tokenizer encodes text into token IDs and decodes token IDs back to text.
type Tokenizer interface {
	Encode(text string) ([]int32, error)
	Decode(tokens []int32) (string, error)
	VocabSize() int
	EOSToken() int32
	BOSToken() int32
}

// NewTokenizerFromGGUF creates a Tokenizer from the vocabulary embedded in a GGUF file.
// It inspects the tokenizer.ggml.model metadata key to choose the right implementation:
//   - "llama" → SentencePiece (unigram)
//   - "gpt2"  → BPE
func NewTokenizerFromGGUF(path string) (Tokenizer, error) {
	// Parse without SkipLargeMetadata so we get the tokenizer arrays.
	gf, err := gguf.ParseGGUFFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse GGUF file: %w", err)
	}

	tokMeta := gf.Tokenizer()

	// Extract vocabulary tokens.
	tokensKV, found := gf.Header.MetadataKV.Get("tokenizer.ggml.tokens")
	if !found {
		return nil, fmt.Errorf("GGUF file missing tokenizer.ggml.tokens")
	}
	tokens := tokensKV.ValueArray().ValuesString()
	if len(tokens) == 0 {
		return nil, fmt.Errorf("GGUF file has empty tokenizer vocabulary")
	}

	// Extract scores (optional — BPE models may not have them).
	var scores []float32
	if scoresKV, ok := gf.Header.MetadataKV.Get("tokenizer.ggml.scores"); ok {
		scores = scoresKV.ValueArray().ValuesFloat32()
	}

	// Extract merges (for BPE models).
	var merges []string
	if mergesKV, ok := gf.Header.MetadataKV.Get("tokenizer.ggml.merges"); ok {
		merges = mergesKV.ValueArray().ValuesString()
	}

	bosID := int32(tokMeta.BOSTokenID)
	eosID := int32(tokMeta.EOSTokenID)

	model := strings.ToLower(tokMeta.Model)
	switch model {
	case "llama":
		return NewSentencePieceTokenizer(tokens, scores, bosID, eosID)
	case "gpt2":
		return NewBPETokenizer(tokens, scores, merges, bosID, eosID)
	default:
		// Fall back to SentencePiece for unknown models that have scores,
		// otherwise use BPE if merges are present.
		if len(scores) > 0 {
			return NewSentencePieceTokenizer(tokens, scores, bosID, eosID)
		}
		if len(merges) > 0 {
			return NewBPETokenizer(tokens, scores, merges, bosID, eosID)
		}
		return nil, fmt.Errorf("unsupported tokenizer model %q (no scores or merges found)", model)
	}
}
