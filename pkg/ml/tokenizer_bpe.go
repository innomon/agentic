package ml

import (
	"fmt"
	"strings"
)

// bpeTokenizer implements the Tokenizer interface using byte-pair encoding.
// It works with GPT-2/Qwen-family models whose merge rules are embedded in GGUF files.
type bpeTokenizer struct {
	idToToken []string
	tokenToID map[string]int32
	mergeRank map[string]int // "tokenA tokenB" → rank (lower = higher priority)
	bosID     int32
	eosID     int32
}

// NewBPETokenizer creates a BPE tokenizer from GGUF vocabulary data.
func NewBPETokenizer(tokens []string, scores []float32, merges []string, bosID, eosID int32) (*bpeTokenizer, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty vocabulary")
	}

	tok := &bpeTokenizer{
		idToToken: tokens,
		tokenToID: make(map[string]int32, len(tokens)),
		mergeRank: make(map[string]int, len(merges)),
		bosID:     bosID,
		eosID:     eosID,
	}

	for i, t := range tokens {
		tok.tokenToID[t] = int32(i)
	}

	for i, m := range merges {
		tok.mergeRank[m] = i
	}

	return tok, nil
}

func (t *bpeTokenizer) VocabSize() int  { return len(t.idToToken) }
func (t *bpeTokenizer) EOSToken() int32 { return t.eosID }
func (t *bpeTokenizer) BOSToken() int32 { return t.bosID }

// Encode tokenizes text using byte-pair encoding.
// Each character is initially treated as a separate token, then merge rules
// are iteratively applied in priority order.
func (t *bpeTokenizer) Encode(text string) ([]int32, error) {
	if text == "" {
		return nil, nil
	}

	// Split text into words on whitespace boundaries, preserving leading spaces.
	words := splitBPEWords(text)

	var allIDs []int32
	for _, word := range words {
		ids := t.encodeWord(word)
		allIDs = append(allIDs, ids...)
	}

	return allIDs, nil
}

// splitBPEWords splits text at whitespace boundaries, attaching each space
// to the following word (GPT-2 style pre-tokenization).
func splitBPEWords(text string) []string {
	var words []string
	var current strings.Builder

	for i, r := range text {
		if r == ' ' && i > 0 && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}
	return words
}

// encodeWord applies BPE merge rules to a single word.
func (t *bpeTokenizer) encodeWord(word string) []int32 {
	if word == "" {
		return nil
	}

	// Check if the whole word is a known token.
	if id, ok := t.tokenToID[word]; ok {
		return []int32{id}
	}

	// Start with individual characters (or UTF-8 runes) as symbols.
	symbols := make([]string, 0, len(word))
	for _, r := range word {
		symbols = append(symbols, string(r))
	}

	// Iteratively apply the highest-priority merge.
	for {
		if len(symbols) < 2 {
			break
		}

		// Find the pair with the lowest merge rank (highest priority).
		bestRank := -1
		bestIdx := -1
		for i := 0; i < len(symbols)-1; i++ {
			pair := symbols[i] + " " + symbols[i+1]
			if rank, ok := t.mergeRank[pair]; ok {
				if bestIdx == -1 || rank < bestRank {
					bestRank = rank
					bestIdx = i
				}
			}
		}

		if bestIdx == -1 {
			break // No more merges possible.
		}

		// Apply the merge: combine symbols[bestIdx] and symbols[bestIdx+1].
		merged := symbols[bestIdx] + symbols[bestIdx+1]
		newSymbols := make([]string, 0, len(symbols)-1)
		newSymbols = append(newSymbols, symbols[:bestIdx]...)
		newSymbols = append(newSymbols, merged)
		newSymbols = append(newSymbols, symbols[bestIdx+2:]...)
		symbols = newSymbols
	}

	// Convert symbols to token IDs.
	ids := make([]int32, 0, len(symbols))
	for _, sym := range symbols {
		if id, ok := t.tokenToID[sym]; ok {
			ids = append(ids, id)
		} else {
			// Byte fallback: encode each byte as a separate token.
			ids = append(ids, t.byteFallback(sym)...)
		}
	}

	return ids
}

// byteFallback encodes a string byte-by-byte using <0xHH> tokens.
func (t *bpeTokenizer) byteFallback(s string) []int32 {
	var ids []int32
	for i := 0; i < len(s); i++ {
		byteToken := fmt.Sprintf("<0x%02X>", s[i])
		if id, ok := t.tokenToID[byteToken]; ok {
			ids = append(ids, id)
			continue
		}
		if id, ok := t.tokenToID[string(s[i])]; ok {
			ids = append(ids, id)
			continue
		}
		if id, ok := t.tokenToID["<unk>"]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// Decode converts token IDs back to text.
func (t *bpeTokenizer) Decode(tokens []int32) (string, error) {
	var b strings.Builder
	for _, id := range tokens {
		if int(id) < 0 || int(id) >= len(t.idToToken) {
			return "", fmt.Errorf("token ID %d out of range [0, %d)", id, len(t.idToToken))
		}
		b.WriteString(t.idToToken[id])
	}
	return b.String(), nil
}
