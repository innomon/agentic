package gomlx

// Arch defines the interface for a transformer architecture implementation.
// Each architecture (LLaMA, Granite Hybrid, etc.) implements this interface
// to provide a forward pass for autoregressive token generation.
type Arch interface {
	// Name returns the architecture name (e.g., "llama", "granitehybrid").
	Name() string
	// NewCacheForModel creates a state cache (KV cache + any recurrent state) sized for this model.
	NewCacheForModel(maxSeqLen int) *KVCache
	// Forward runs a forward pass for the given input token IDs starting at position pos.
	// Returns logits of shape [vocabSize].
	Forward(inputIDs []int32, pos int, kv *KVCache) []float32
}
