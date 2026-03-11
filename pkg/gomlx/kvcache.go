package gomlx

// MambaLayerState holds the recurrent state for a single Mamba2 layer.
type MambaLayerState struct {
	ConvState []float32 // [convDim * (kernel-1)] shift register for causal conv1d
	SSMState  []float32 // [nHeads * headDim * dState] recurrent SSM state
}

// MambaCache holds state for all Mamba2 layers in a hybrid model.
type MambaCache struct {
	Layers []MambaLayerState // indexed by global layer index
}

// KVCache holds key-value cache state for transformer attention layers.
// Keys and Values are stored as flat slices per layer per head:
//
//	Keys[layer][head] contains SeqLen * HeadDim float32 values.
type KVCache struct {
	Keys   [][][]float32 // [nLayers][nKVHeads][seqLen * headDim]
	Values [][][]float32

	Mamba *MambaCache

	SeqLen int

	NLayers   int
	NKVHeads  int
	HeadDim   int
	MaxSeqLen int
}

// NewKVCache allocates an empty KV cache with the given dimensions.
func NewKVCache(nLayers, nKVHeads, headDim, maxSeqLen int) *KVCache {
	kv := &KVCache{
		Keys:      make([][][]float32, nLayers),
		Values:    make([][][]float32, nLayers),
		NLayers:   nLayers,
		NKVHeads:  nKVHeads,
		HeadDim:   headDim,
		MaxSeqLen: maxSeqLen,
	}
	for l := 0; l < nLayers; l++ {
		kv.Keys[l] = make([][]float32, nKVHeads)
		kv.Values[l] = make([][]float32, nKVHeads)
		for h := 0; h < nKVHeads; h++ {
			kv.Keys[l][h] = make([]float32, 0, maxSeqLen*headDim)
			kv.Values[l][h] = make([]float32, 0, maxSeqLen*headDim)
		}
	}
	return kv
}

// Update appends new key/value entries for a single position to the cache for the given layer.
// keys and values each have shape [nKVHeads * headDim].
func (kv *KVCache) Update(layer int, keys, values []float32) {
	for h := 0; h < kv.NKVHeads; h++ {
		off := h * kv.HeadDim
		kv.Keys[layer][h] = append(kv.Keys[layer][h], keys[off:off+kv.HeadDim]...)
		kv.Values[layer][h] = append(kv.Values[layer][h], values[off:off+kv.HeadDim]...)
	}
}

// KeysForHead returns the cached keys for a given layer and head as a flat slice.
// The slice contains SeqLen * HeadDim elements.
func (kv *KVCache) KeysForHead(layer, head int) []float32 {
	return kv.Keys[layer][head]
}

// ValuesForHead returns the cached values for a given layer and head as a flat slice.
func (kv *KVCache) ValuesForHead(layer, head int) []float32 {
	return kv.Values[layer][head]
}

// CurrentSeqLen returns the current sequence length stored in the cache for layer 0, head 0.
func (kv *KVCache) CurrentSeqLen() int {
	if kv.NLayers == 0 || kv.NKVHeads == 0 {
		return 0
	}
	return len(kv.Keys[0][0]) / kv.HeadDim
}

// Reset clears all cached entries.
func (kv *KVCache) Reset() {
	for l := 0; l < kv.NLayers; l++ {
		for h := 0; h < kv.NKVHeads; h++ {
			kv.Keys[l][h] = kv.Keys[l][h][:0]
			kv.Values[l][h] = kv.Values[l][h][:0]
		}
	}
	if kv.Mamba != nil {
		for i := range kv.Mamba.Layers {
			for j := range kv.Mamba.Layers[i].ConvState {
				kv.Mamba.Layers[i].ConvState[j] = 0
			}
			for j := range kv.Mamba.Layers[i].SSMState {
				kv.Mamba.Layers[i].SSMState[j] = 0
			}
		}
	}
}
