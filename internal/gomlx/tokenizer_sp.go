package gomlx

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// spTokenizer implements the Tokenizer interface using a unigram (SentencePiece)
// model built from vocabulary tokens and their scores extracted from a GGUF file.
type spTokenizer struct {
	idToToken []string
	tokenToID map[string]int32
	scores    []float32
	bosID     int32
	eosID     int32
}

// NewSentencePieceTokenizer creates a unigram tokenizer from GGUF vocabulary data.
func NewSentencePieceTokenizer(tokens []string, scores []float32, bosID, eosID int32) (*spTokenizer, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty vocabulary")
	}

	// Pad scores with zeros if shorter than tokens (some models omit trailing zeros).
	if len(scores) < len(tokens) {
		padded := make([]float32, len(tokens))
		copy(padded, scores)
		scores = padded
	}

	tok := &spTokenizer{
		idToToken: tokens,
		tokenToID: make(map[string]int32, len(tokens)),
		scores:    scores,
		bosID:     bosID,
		eosID:     eosID,
	}

	for i, t := range tokens {
		tok.tokenToID[t] = int32(i)
	}

	return tok, nil
}

func (t *spTokenizer) VocabSize() int  { return len(t.idToToken) }
func (t *spTokenizer) EOSToken() int32 { return t.eosID }
func (t *spTokenizer) BOSToken() int32 { return t.bosID }

// Encode tokenizes text using a unigram Viterbi forward algorithm.
// SentencePiece replaces leading spaces with the meta-symbol ▁ (U+2581).
func (t *spTokenizer) Encode(text string) ([]int32, error) {
	if text == "" {
		return nil, nil
	}

	// SentencePiece convention: prepend space and replace spaces with ▁.
	text = "▁" + strings.ReplaceAll(text, " ", "▁")

	ids := t.viterbiEncode(text)
	return ids, nil
}

// viterbiEncode finds the highest-scoring tokenization using dynamic programming.
func (t *spTokenizer) viterbiEncode(text string) []int32 {
	n := len(text) // byte length
	if n == 0 {
		return nil
	}

	// best[i] = best log-probability of tokenizing text[:i]
	// prev[i] = the start byte position of the token ending at position i
	const negInf = -float64(math.MaxFloat64)
	best := make([]float64, n+1)
	prev := make([]int, n+1)
	bestID := make([]int32, n+1)

	for i := range best {
		best[i] = negInf
		prev[i] = -1
		bestID[i] = -1
	}
	best[0] = 0

	for i := 0; i < n; {
		if best[i] == negInf {
			// Skip unreachable positions (shouldn't happen with byte fallback).
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
			continue
		}

		// Try all substrings starting at position i.
		for j := i + 1; j <= n; j++ {
			substr := text[i:j]
			id, ok := t.tokenToID[substr]
			if !ok {
				continue
			}
			score := float64(t.scores[id])
			candidate := best[i] + score
			if candidate > best[j] {
				best[j] = candidate
				prev[j] = i
				bestID[j] = id
			}
		}

		// Advance by one rune.
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}

	// If we couldn't reach the end, fall back to byte-level encoding.
	if best[n] == negInf {
		return t.byteFallbackEncode(text)
	}

	// Backtrack to collect tokens.
	var ids []int32
	for pos := n; pos > 0; {
		ids = append(ids, bestID[pos])
		pos = prev[pos]
	}

	// Reverse.
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}

	return ids
}

// byteFallbackEncode encodes each byte individually using <0xHH> byte tokens
// or single-character tokens, whichever is available.
func (t *spTokenizer) byteFallbackEncode(text string) []int32 {
	var ids []int32
	for i := 0; i < len(text); i++ {
		b := text[i]
		// Try byte token format used by llama models.
		byteToken := fmt.Sprintf("<0x%02X>", b)
		if id, ok := t.tokenToID[byteToken]; ok {
			ids = append(ids, id)
			continue
		}
		// Try single byte as string.
		if id, ok := t.tokenToID[string(b)]; ok {
			ids = append(ids, id)
			continue
		}
		// Unknown byte — use unknown token if available.
		if id, ok := t.tokenToID["<unk>"]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// Decode converts token IDs back to text. It replaces ▁ with spaces
// and trims the leading space that Encode prepends.
func (t *spTokenizer) Decode(tokens []int32) (string, error) {
	var b strings.Builder
	for _, id := range tokens {
		if int(id) < 0 || int(id) >= len(t.idToToken) {
			return "", fmt.Errorf("token ID %d out of range [0, %d)", id, len(t.idToToken))
		}
		b.WriteString(t.idToToken[id])
	}
	result := b.String()
	result = strings.ReplaceAll(result, "▁", " ")
	// SentencePiece prepends a space — strip the leading one.
	result = strings.TrimPrefix(result, " ")
	return result, nil
}
