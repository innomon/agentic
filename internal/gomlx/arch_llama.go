package gomlx

import (
	"fmt"
	"math"
)

// LlamaArch implements a pure-Go LLaMA transformer for single-token decode inference.
type LlamaArch struct {
	weights      WeightMap
	info         *GGUFModelInfo
	embedDim     int
	nHeads       int
	nKVHeads     int
	headDim      int
	nLayers      int
	vocabSize    int
	ffDim        int
	rmsEps       float32
	ropeFreqBase float32
}

// NewLlamaArch creates a LLaMA architecture instance from loaded weights and GGUF metadata.
func NewLlamaArch(weights WeightMap, info *GGUFModelInfo) (*LlamaArch, error) {
	if info == nil {
		return nil, fmt.Errorf("GGUFModelInfo is nil")
	}

	embedDim := int(info.EmbeddingLength)
	nHeads := int(info.AttentionHeadCount)
	nKVHeads := int(info.AttentionHeadKV)
	nLayers := int(info.BlockCount)
	vocabSize := int(info.VocabSize)

	if embedDim == 0 || nHeads == 0 || nLayers == 0 || vocabSize == 0 {
		return nil, fmt.Errorf("incomplete model info: embed=%d heads=%d layers=%d vocab=%d",
			embedDim, nHeads, nLayers, vocabSize)
	}

	if nKVHeads == 0 {
		nKVHeads = nHeads // MHA fallback
	}

	headDim := embedDim / nHeads

	var ffDim int
	if len(info.FeedForwardLength) > 0 {
		ffDim = int(info.FeedForwardLength[0])
	}
	if ffDim == 0 {
		// Infer from gate weight shape.
		if w, ok := weights["blk.0.ffn_gate.weight"]; ok && len(w.Shape) >= 1 {
			ffDim = w.Shape[0]
		}
	}
	if ffDim == 0 {
		return nil, fmt.Errorf("cannot determine feed-forward dimension")
	}

	// Validate critical weights exist.
	required := []string{"token_embd.weight", "output_norm.weight"}
	for _, name := range required {
		if _, ok := weights[name]; !ok {
			return nil, fmt.Errorf("missing required weight: %s", name)
		}
	}

	// Default RoPE frequency base.
	var ropeFreqBase float32 = 10000.0

	// Default RMS epsilon.
	var rmsEps float32 = 1e-5

	return &LlamaArch{
		weights:      weights,
		info:         info,
		embedDim:     embedDim,
		nHeads:       nHeads,
		nKVHeads:     nKVHeads,
		headDim:      headDim,
		nLayers:      nLayers,
		vocabSize:    vocabSize,
		ffDim:        ffDim,
		rmsEps:       rmsEps,
		ropeFreqBase: ropeFreqBase,
	}, nil
}

// Name returns the architecture name.
func (l *LlamaArch) Name() string { return "llama" }

// NewKVCacheForModel creates a KV cache sized for this model.
func (l *LlamaArch) NewKVCacheForModel(maxSeqLen int) *KVCache {
	return NewKVCache(l.nLayers, l.nKVHeads, l.headDim, maxSeqLen)
}

// Forward runs a single-token forward pass.
// inputIDs contains token IDs to process (typically one for decode mode).
// pos is the starting position in the sequence.
// kv is the key-value cache (must be pre-allocated).
// Returns logits of shape [vocabSize].
func (l *LlamaArch) Forward(inputIDs []int32, pos int, kv *KVCache) []float32 {
	embWeight := l.weights["token_embd.weight"].Data

	// Process one token at a time (decode mode).
	// For prefill, the caller loops over tokens.
	var hidden []float32

	for ti, tokenID := range inputIDs {
		curPos := pos + ti

		// 1. Embed token: extract row from [vocabSize, embedDim].
		x := make([]float32, l.embedDim)
		copy(x, embWeight[int(tokenID)*l.embedDim:(int(tokenID)+1)*l.embedDim])

		// 2. Transformer blocks.
		for layer := 0; layer < l.nLayers; layer++ {
			x = l.transformerBlock(layer, x, curPos, kv)
		}

		hidden = x
	}

	// 3. Final RMS norm.
	normWeight := l.weights["output_norm.weight"].Data
	normed := make([]float32, l.embedDim)
	RMSNorm(normed, hidden, normWeight, l.rmsEps)

	// 4. Output projection → logits.
	var outWeight []float32
	if w, ok := l.weights["output.weight"]; ok {
		outWeight = w.Data
	} else {
		// Weight tying: use token embedding.
		outWeight = embWeight
	}
	logits := MatVecMul(outWeight, normed, l.vocabSize, l.embedDim)

	return logits
}

// transformerBlock runs one transformer block and returns the updated hidden state.
func (l *LlamaArch) transformerBlock(layer int, x []float32, pos int, kv *KVCache) []float32 {
	prefix := fmt.Sprintf("blk.%d.", layer)

	// --- Attention ---
	// a. RMSNorm (attention norm)
	attnNormW := l.weights[prefix+"attn_norm.weight"].Data
	normed := make([]float32, l.embedDim)
	RMSNorm(normed, x, attnNormW, l.rmsEps)

	// b. Q, K, V projections
	qWeight := l.weights[prefix+"attn_q.weight"].Data
	kWeight := l.weights[prefix+"attn_k.weight"].Data
	vWeight := l.weights[prefix+"attn_v.weight"].Data

	qDim := l.nHeads * l.headDim
	kvDim := l.nKVHeads * l.headDim

	q := MatVecMul(qWeight, normed, qDim, l.embedDim)
	k := MatVecMul(kWeight, normed, kvDim, l.embedDim)
	v := MatVecMul(vWeight, normed, kvDim, l.embedDim)

	// c. Apply RoPE to Q and K
	RoPE(q, k, l.headDim, pos, l.nHeads, l.nKVHeads, l.ropeFreqBase)

	// d. Update KV cache
	kv.Update(layer, k, v)

	// e. Compute grouped-query attention
	attnOut := l.groupedQueryAttention(layer, q, kv)

	// f. Output projection
	oWeight := l.weights[prefix+"attn_output.weight"].Data
	attnProj := MatVecMul(oWeight, attnOut, l.embedDim, l.embedDim)

	// g. Residual connection
	residual := make([]float32, l.embedDim)
	Add(residual, x, attnProj)

	// --- Feed-Forward Network ---
	// h. RMSNorm (FFN norm)
	ffnNormW := l.weights[prefix+"ffn_norm.weight"].Data
	ffnNormed := make([]float32, l.embedDim)
	RMSNorm(ffnNormed, residual, ffnNormW, l.rmsEps)

	// i. SwiGLU FFN: down(silu(gate(x)) * up(x))
	gateWeight := l.weights[prefix+"ffn_gate.weight"].Data
	upWeight := l.weights[prefix+"ffn_up.weight"].Data
	downWeight := l.weights[prefix+"ffn_down.weight"].Data

	gate := MatVecMul(gateWeight, ffnNormed, l.ffDim, l.embedDim)
	up := MatVecMul(upWeight, ffnNormed, l.ffDim, l.embedDim)

	SiLU(gate)
	ElemMul(gate, gate, up)

	ffnOut := MatVecMul(downWeight, gate, l.embedDim, l.ffDim)

	// j. Residual connection
	out := make([]float32, l.embedDim)
	Add(out, residual, ffnOut)

	return out
}

// groupedQueryAttention computes multi-head attention with GQA support.
// q has shape [nHeads * headDim]. KV cache is read for all past + current positions.
func (l *LlamaArch) groupedQueryAttention(layer int, q []float32, kv *KVCache) []float32 {
	seqLen := len(kv.Keys[layer][0]) / l.headDim
	kvGroupSize := l.nHeads / l.nKVHeads
	scale := float32(1.0 / math.Sqrt(float64(l.headDim)))

	out := make([]float32, l.nHeads*l.headDim)

	for h := 0; h < l.nHeads; h++ {
		kvHead := h / kvGroupSize
		qVec := q[h*l.headDim : (h+1)*l.headDim]
		cachedK := kv.KeysForHead(layer, kvHead)
		cachedV := kv.ValuesForHead(layer, kvHead)

		// Compute attention scores: Q · K^T for each cached position.
		scores := make([]float32, seqLen)
		for s := 0; s < seqLen; s++ {
			kVec := cachedK[s*l.headDim : (s+1)*l.headDim]
			var dot float32
			for d := 0; d < l.headDim; d++ {
				dot += qVec[d] * kVec[d]
			}
			scores[s] = dot * scale
		}

		// Softmax over scores.
		Softmax(scores)

		// Weighted sum of values.
		headOut := out[h*l.headDim : (h+1)*l.headDim]
		for s := 0; s < seqLen; s++ {
			vVec := cachedV[s*l.headDim : (s+1)*l.headDim]
			w := scores[s]
			for d := 0; d < l.headDim; d++ {
				headOut[d] += w * vVec[d]
			}
		}
	}

	return out
}
