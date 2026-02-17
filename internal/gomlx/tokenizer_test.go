package gomlx

import (
	"os"
	"testing"
)

func TestSentencePieceTokenizer(t *testing.T) {
	// Build a small synthetic vocabulary mimicking SentencePiece conventions.
	tokens := []string{
		"<unk>",  // 0
		"<s>",    // 1 (BOS)
		"</s>",   // 2 (EOS)
		"▁",      // 3
		"▁Hello", // 4
		"▁world", // 5
		",",      // 6
		"!",      // 7
		"▁the",   // 8
		"▁a",     // 9
		"H",      // 10
		"e",      // 11
		"l",      // 12
		"o",      // 13
	}
	scores := []float32{
		0,    // <unk>
		0,    // <s>
		0,    // </s>
		-1,   // ▁
		-2,   // ▁Hello
		-2,   // ▁world
		-3,   // ,
		-3,   // !
		-2.5, // ▁the
		-2.5, // ▁a
		-4,   // H
		-4,   // e
		-4,   // l
		-4,   // o
	}

	tok, err := NewSentencePieceTokenizer(tokens, scores, 1, 2)
	if err != nil {
		t.Fatalf("NewSentencePieceTokenizer: %v", err)
	}

	if tok.VocabSize() != len(tokens) {
		t.Errorf("VocabSize = %d, want %d", tok.VocabSize(), len(tokens))
	}
	if tok.BOSToken() != 1 {
		t.Errorf("BOSToken = %d, want 1", tok.BOSToken())
	}
	if tok.EOSToken() != 2 {
		t.Errorf("EOSToken = %d, want 2", tok.EOSToken())
	}

	// Encode "Hello, world!" → ▁Hello, ▁world!
	ids, err := tok.Encode("Hello, world!")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("Encode returned empty tokens")
	}
	t.Logf("Encode(\"Hello, world!\") = %v", ids)

	// Decode back.
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	t.Logf("Decode(%v) = %q", ids, decoded)

	if decoded != "Hello, world!" {
		t.Errorf("round-trip failed: got %q, want %q", decoded, "Hello, world!")
	}

	// Empty input.
	emptyIDs, err := tok.Encode("")
	if err != nil {
		t.Fatalf("Encode empty: %v", err)
	}
	if len(emptyIDs) != 0 {
		t.Errorf("Encode(\"\") = %v, want empty", emptyIDs)
	}
}

func TestSentencePieceTokenizerByteFallback(t *testing.T) {
	// A tiny vocab with byte tokens for fallback.
	tokens := []string{
		"<unk>",   // 0
		"<s>",     // 1
		"</s>",    // 2
		"<0x48>",  // 3 — 'H'
		"<0x69>",  // 4 — 'i'
		"▁",       // 5
		"<0xE2>",  // 6
		"<0x96>",  // 7
		"<0x81>",  // 8
	}
	scores := make([]float32, len(tokens))

	tok, err := NewSentencePieceTokenizer(tokens, scores, 1, 2)
	if err != nil {
		t.Fatalf("NewSentencePieceTokenizer: %v", err)
	}

	// "Hi" should use byte fallback since ▁Hi isn't in vocab.
	// The text becomes "▁Hi" → ▁ + H + i (byte tokens).
	ids, err := tok.Encode("Hi")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	t.Logf("Encode(\"Hi\") with byte fallback = %v", ids)
	if len(ids) == 0 {
		t.Error("expected non-empty token list from byte fallback")
	}
}

func TestBPETokenizer(t *testing.T) {
	// Build a synthetic BPE vocabulary.
	tokens := []string{
		"<unk>", // 0
		"<s>",   // 1 (BOS)
		"</s>",  // 2 (EOS)
		"H",     // 3
		"e",     // 4
		"l",     // 5
		"o",     // 6
		" ",     // 7
		"w",     // 8
		"r",     // 9
		"d",     // 10
		"!",     // 11
		"He",    // 12
		"ll",    // 13
		"lo",    // 14
		"Hell",  // 15
		"Hello", // 16
		",",     // 17
	}
	scores := make([]float32, len(tokens))

	// Merges define the order: lower index = applied first.
	merges := []string{
		"H e",    // → "He"
		"l l",    // → "ll"
		"l o",    // → "lo"
		"He ll",  // → "Hell"
		"Hell o", // → "Hello"
	}

	tok, err := NewBPETokenizer(tokens, scores, merges, 1, 2)
	if err != nil {
		t.Fatalf("NewBPETokenizer: %v", err)
	}

	if tok.VocabSize() != len(tokens) {
		t.Errorf("VocabSize = %d, want %d", tok.VocabSize(), len(tokens))
	}

	// "Hello" should merge fully into token 16.
	ids, err := tok.Encode("Hello")
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	t.Logf("Encode(\"Hello\") = %v", ids)

	if len(ids) != 1 || ids[0] != 16 {
		t.Errorf("Encode(\"Hello\") = %v, want [16]", ids)
	}

	// Decode back.
	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded != "Hello" {
		t.Errorf("Decode = %q, want %q", decoded, "Hello")
	}

	// Multi-word encoding.
	ids2, err := tok.Encode("Hello, world!")
	if err != nil {
		t.Fatalf("Encode multi-word: %v", err)
	}
	t.Logf("Encode(\"Hello, world!\") = %v", ids2)
	if len(ids2) == 0 {
		t.Error("expected non-empty tokens for multi-word input")
	}

	decoded2, err := tok.Decode(ids2)
	if err != nil {
		t.Fatalf("Decode multi-word: %v", err)
	}
	t.Logf("Decode = %q", decoded2)
	if decoded2 != "Hello, world!" {
		t.Errorf("round-trip failed: got %q, want %q", decoded2, "Hello, world!")
	}

	// Empty input.
	emptyIDs, err := tok.Encode("")
	if err != nil {
		t.Fatalf("Encode empty: %v", err)
	}
	if len(emptyIDs) != 0 {
		t.Errorf("Encode(\"\") = %v, want empty", emptyIDs)
	}
}

func TestBPETokenizerDecodeOutOfRange(t *testing.T) {
	tokens := []string{"a", "b", "c"}
	tok, err := NewBPETokenizer(tokens, nil, nil, -1, -1)
	if err != nil {
		t.Fatalf("NewBPETokenizer: %v", err)
	}

	_, err = tok.Decode([]int32{99})
	if err == nil {
		t.Error("expected error for out-of-range token ID")
	}
}

func TestSentencePieceDecodeOutOfRange(t *testing.T) {
	tokens := []string{"a", "b", "c"}
	tok, err := NewSentencePieceTokenizer(tokens, nil, -1, -1)
	if err != nil {
		t.Fatalf("NewSentencePieceTokenizer: %v", err)
	}

	_, err = tok.Decode([]int32{99})
	if err == nil {
		t.Error("expected error for out-of-range token ID")
	}
}

func TestTokenizerFromGGUF(t *testing.T) {
	modelPath := os.Getenv("GOMLX_TEST_MODEL")
	if modelPath == "" {
		if _, err := os.Stat("testdata/test.gguf"); err == nil {
			modelPath = "testdata/test.gguf"
		}
	}
	if modelPath == "" {
		t.Skip("set GOMLX_TEST_MODEL to a .gguf file path to run this test")
	}

	tok, err := NewTokenizerFromGGUF(modelPath)
	if err != nil {
		t.Fatalf("NewTokenizerFromGGUF: %v", err)
	}

	if tok.VocabSize() <= 0 {
		t.Error("expected positive vocab size")
	}
	t.Logf("VocabSize: %d", tok.VocabSize())
	t.Logf("BOSToken: %d", tok.BOSToken())
	t.Logf("EOSToken: %d", tok.EOSToken())

	// Encode/decode round trip.
	text := "Hello, world!"
	ids, err := tok.Encode(text)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("Encode returned empty tokens")
	}
	t.Logf("Encode(%q) = %v (%d tokens)", text, ids, len(ids))

	decoded, err := tok.Decode(ids)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	t.Logf("Decode = %q", decoded)

	// The decoded text should contain the original words.
	if !containsAll(decoded, "Hello", "world") {
		t.Errorf("decoded text %q doesn't contain expected words", decoded)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
