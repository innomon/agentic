package ml

import (
	"fmt"
	"math"
)

// mambaLayerParams holds inferred Mamba2 dimensions for a single layer.
type mambaLayerParams struct {
	dInner       int // intermediate_size (gate + hidden dims)
	convDim      int // dInner + 2*nGroups*dState
	kernel       int // conv1d kernel size (typically 4)
	nHeads       int // number of SSM heads
	dState       int // SSM state dimension per head
	nGroups      int // number of groups for B/C
	innerHeadDim int // dInner / nHeads
}

// GraniteHybridArch implements a hybrid Mamba2 + Attention transformer.
// Layers are mixed according to layerTypes: "attention" uses GQA, "mamba" uses Mamba2 SSM.
type GraniteHybridArch struct {
	weights WeightMap
	info    *GGUFModelInfo

	embedDim  int
	nLayers   int
	vocabSize int

	// Attention dims (for attention layers).
	nHeads   int
	nKVHeads int
	headDim  int

	// FFN dims.
	ffDim  int
	rmsEps float32

	// Per-layer type: "attention" or "mamba".
	layerTypes []string

	// Mamba2 params (shared across mamba layers; inferred from first mamba layer).
	mamba *mambaLayerParams

	// Scaling factors from config.
	embeddingMultiplier float32
	residualMultiplier  float32
	logitsScaling       float32

	// RoPE frequency base (used only for attention layers, 0 means NoPE).
	ropeFreqBase float32
	useRoPE      bool
}

// NewGraniteHybridArch creates a Granite Hybrid architecture from loaded weights and GGUF metadata.
func NewGraniteHybridArch(weights WeightMap, info *GGUFModelInfo) (*GraniteHybridArch, error) {
	if info == nil {
		return nil, fmt.Errorf("GGUFModelInfo is nil")
	}

	embedDim := int(info.EmbeddingLength)
	nHeads := int(info.AttentionHeadCount)
	nKVHeads := int(info.AttentionHeadKV)
	nLayers := int(info.BlockCount)
	vocabSize := int(info.VocabSize)

	if embedDim == 0 || nLayers == 0 || vocabSize == 0 {
		return nil, fmt.Errorf("incomplete model info: embed=%d layers=%d vocab=%d",
			embedDim, nLayers, vocabSize)
	}

	if nKVHeads == 0 {
		nKVHeads = nHeads
	}

	headDim := 0
	if nHeads > 0 {
		headDim = embedDim / nHeads
	}

	// Determine FFN dimension.
	var ffDim int
	if len(info.FeedForwardLength) > 0 {
		ffDim = int(info.FeedForwardLength[0])
	}
	if ffDim == 0 {
		// Infer from first attention layer's gate weight or first available.
		for i := 0; i < nLayers; i++ {
			key := fmt.Sprintf("blk.%d.ffn_gate.weight", i)
			if w, ok := weights[key]; ok && len(w.Shape) >= 1 {
				ffDim = w.Shape[0]
				break
			}
		}
	}

	// Determine layer types by probing tensor existence.
	layerTypes := make([]string, nLayers)
	for i := 0; i < nLayers; i++ {
		ssmKey := fmt.Sprintf("blk.%d.ssm_in.weight", i)
		attnKey := fmt.Sprintf("blk.%d.attn_q.weight", i)
		hasMamba := false
		hasAttn := false
		if _, ok := weights[ssmKey]; ok {
			hasMamba = true
		}
		if _, ok := weights[attnKey]; ok {
			hasAttn = true
		}
		if hasMamba {
			layerTypes[i] = "mamba"
		} else if hasAttn {
			layerTypes[i] = "attention"
		} else {
			return nil, fmt.Errorf("layer %d has neither attention nor mamba weights", i)
		}
	}

	// Infer Mamba2 parameters from the first mamba layer.
	var mParams *mambaLayerParams
	for i := 0; i < nLayers; i++ {
		if layerTypes[i] != "mamba" {
			continue
		}
		mp, err := inferMambaParams(weights, i, embedDim)
		if err != nil {
			return nil, fmt.Errorf("infer mamba params from layer %d: %w", i, err)
		}
		mParams = mp
		break
	}

	// Determine if RoPE is used (hybrid models typically use NoPE).
	useRoPE := true
	hasMamba := mParams != nil
	if hasMamba {
		// Granite hybrid models use NoPE.
		useRoPE = false
	}

	var ropeFreqBase float32
	if useRoPE {
		ropeFreqBase = 10000.0
	}

	// Scaling factors — infer from GGUF metadata if available, otherwise defaults.
	var embeddingMultiplier float32 = 1.0
	var residualMultiplier float32 = 1.0
	var logitsScaling float32 = 1.0

	// Try to read scaling factors from metadata stored in weights or info.
	// GGUF stores these as metadata keys; we read them from info if available.
	if info.EmbeddingScale != 0 {
		embeddingMultiplier = info.EmbeddingScale
	}
	if info.ResidualScale != 0 {
		residualMultiplier = info.ResidualScale
	}
	if info.LogitScale != 0 {
		logitsScaling = info.LogitScale
	}

	// Default RMS epsilon.
	var rmsEps float32 = 1e-5

	// Validate critical weights exist.
	if _, ok := weights["token_embd.weight"]; !ok {
		return nil, fmt.Errorf("missing required weight: token_embd.weight")
	}
	if _, ok := weights["output_norm.weight"]; !ok {
		return nil, fmt.Errorf("missing required weight: output_norm.weight")
	}

	return &GraniteHybridArch{
		weights:             weights,
		info:                info,
		embedDim:            embedDim,
		nLayers:             nLayers,
		vocabSize:           vocabSize,
		nHeads:              nHeads,
		nKVHeads:            nKVHeads,
		headDim:             headDim,
		ffDim:               ffDim,
		rmsEps:              rmsEps,
		layerTypes:          layerTypes,
		mamba:               mParams,
		embeddingMultiplier: embeddingMultiplier,
		residualMultiplier:  residualMultiplier,
		logitsScaling:       logitsScaling,
		ropeFreqBase:        ropeFreqBase,
		useRoPE:             useRoPE,
	}, nil
}

// inferMambaParams infers Mamba2 dimensions from the weight shapes of a given layer.
func inferMambaParams(weights WeightMap, layer, embedDim int) (*mambaLayerParams, error) {
	prefix := fmt.Sprintf("blk.%d.", layer)

	wIn, ok := weights[prefix+"ssm_in.weight"]
	if !ok {
		return nil, fmt.Errorf("missing %sssm_in.weight", prefix)
	}
	wConv, ok := weights[prefix+"ssm_conv1d.weight"]
	if !ok {
		return nil, fmt.Errorf("missing %sssm_conv1d.weight", prefix)
	}
	bDt, ok := weights[prefix+"ssm_dt.bias"]
	if !ok {
		return nil, fmt.Errorf("missing %sssm_dt.bias", prefix)
	}
	wA, ok := weights[prefix+"ssm_a"]
	if !ok {
		return nil, fmt.Errorf("missing %sssm_a", prefix)
	}

	// in_proj output: [projectionSize, embedDim] where projectionSize = dInner + convDim + nHeads
	projectionSize := wIn.Shape[0]

	// conv1d weight: GGUF stores depthwise conv as [convDim, kernel]
	convDim := wConv.Shape[0]
	kernel := 1
	if len(wConv.Shape) > 1 {
		kernel = wConv.Shape[1]
	}

	// dt_bias shape: [nHeads]
	nHeads := bDt.Shape[0]

	// A shape: varies. Could be [1, nHeads] or [nHeads] (scalar per head, dState=1 implicit)
	// or [nHeads, dState]. In GGUF, -exp(A_log) is pre-applied.
	var dState int
	if len(wA.Shape) >= 2 {
		// Find the state dimension (not 1, not nHeads).
		for _, d := range wA.Shape {
			if d != 1 && d != nHeads {
				dState = d
				break
			}
		}
	}
	if dState == 0 {
		// A is per-head scalar; compute dState from other dimensions.
		// convDim = dInner + 2*nGroups*dState, and dInner = projectionSize - convDim - nHeads
		// We need another source. Try ssm_d weight or infer from typical values.
		// Fallback: use total elements of A divided by nHeads.
		totalA := 1
		for _, d := range wA.Shape {
			totalA *= d
		}
		dState = totalA / nHeads
		if dState < 1 {
			dState = 1
		}
	}

	dInner := projectionSize - convDim - nHeads
	if dInner <= 0 {
		return nil, fmt.Errorf("invalid mamba dims: projSize=%d convDim=%d nHeads=%d → dInner=%d",
			projectionSize, convDim, nHeads, dInner)
	}

	// Infer nGroups from convDim - dInner = 2*nGroups*dState.
	bcDim := convDim - dInner
	if bcDim <= 0 || bcDim%(2*dState) != 0 {
		return nil, fmt.Errorf("invalid B/C dims: convDim=%d dInner=%d dState=%d → bcDim=%d",
			convDim, dInner, dState, bcDim)
	}
	nGroups := bcDim / (2 * dState)

	if dInner%nHeads != 0 {
		return nil, fmt.Errorf("dInner=%d not divisible by nHeads=%d", dInner, nHeads)
	}
	innerHeadDim := dInner / nHeads

	return &mambaLayerParams{
		dInner:       dInner,
		convDim:      convDim,
		kernel:       kernel,
		nHeads:       nHeads,
		dState:       dState,
		nGroups:      nGroups,
		innerHeadDim: innerHeadDim,
	}, nil
}

// Name returns the architecture name.
func (g *GraniteHybridArch) Name() string { return "granitehybrid" }

// NewCacheForModel creates a combined KV + Mamba cache sized for this model.
func (g *GraniteHybridArch) NewCacheForModel(maxSeqLen int) *KVCache {
	kv := NewKVCache(g.nLayers, g.nKVHeads, g.headDim, maxSeqLen)

	if g.mamba != nil {
		mc := &MambaCache{
			Layers: make([]MambaLayerState, g.nLayers),
		}
		for i := 0; i < g.nLayers; i++ {
			if g.layerTypes[i] == "mamba" {
				histLen := g.mamba.kernel - 1
				mc.Layers[i] = MambaLayerState{
					ConvState: make([]float32, g.mamba.convDim*histLen),
					SSMState:  make([]float32, g.mamba.dInner*g.mamba.dState),
				}
			}
		}
		kv.Mamba = mc
	}

	return kv
}

// Forward runs a forward pass for the given input tokens.
func (g *GraniteHybridArch) Forward(inputIDs []int32, pos int, kv *KVCache) []float32 {
	embWeight := g.weights["token_embd.weight"].Data

	var hidden []float32

	for ti, tokenID := range inputIDs {
		curPos := pos + ti

		// 1. Embed token and apply embedding multiplier.
		x := make([]float32, g.embedDim)
		copy(x, embWeight[int(tokenID)*g.embedDim:(int(tokenID)+1)*g.embedDim])
		if g.embeddingMultiplier != 1.0 {
			for i := range x {
				x[i] *= g.embeddingMultiplier
			}
		}

		// 2. Transformer blocks.
		for layer := 0; layer < g.nLayers; layer++ {
			if g.layerTypes[layer] == "mamba" {
				x = g.mambaBlock(layer, x, kv)
			} else {
				x = g.attentionBlock(layer, x, curPos, kv)
			}
			x = g.ffnBlock(layer, x)
		}

		hidden = x
	}

	// 3. Final RMS norm.
	normWeight := g.weights["output_norm.weight"].Data
	normed := make([]float32, g.embedDim)
	RMSNorm(normed, hidden, normWeight, g.rmsEps)

	// 4. Output projection → logits.
	var outWeight []float32
	if w, ok := g.weights["output.weight"]; ok {
		outWeight = w.Data
	} else {
		outWeight = embWeight // weight tying
	}
	logits := MatVecMul(outWeight, normed, g.vocabSize, g.embedDim)

	// 5. Apply logits scaling.
	if g.logitsScaling != 1.0 {
		invScale := 1.0 / g.logitsScaling
		for i := range logits {
			logits[i] *= invScale
		}
	}

	return logits
}

// attentionBlock runs a standard GQA attention block (optionally with RoPE).
func (g *GraniteHybridArch) attentionBlock(layer int, x []float32, pos int, kv *KVCache) []float32 {
	prefix := fmt.Sprintf("blk.%d.", layer)

	// a. RMSNorm (attention norm).
	attnNormW := g.weights[prefix+"attn_norm.weight"].Data
	normed := make([]float32, g.embedDim)
	RMSNorm(normed, x, attnNormW, g.rmsEps)

	// b. Q, K, V projections.
	qWeight := g.weights[prefix+"attn_q.weight"].Data
	kWeight := g.weights[prefix+"attn_k.weight"].Data
	vWeight := g.weights[prefix+"attn_v.weight"].Data

	qDim := g.nHeads * g.headDim
	kvDim := g.nKVHeads * g.headDim

	q := MatVecMul(qWeight, normed, qDim, g.embedDim)
	k := MatVecMul(kWeight, normed, kvDim, g.embedDim)
	v := MatVecMul(vWeight, normed, kvDim, g.embedDim)

	// c. Apply RoPE if enabled.
	if g.useRoPE {
		RoPE(q, k, g.headDim, pos, g.nHeads, g.nKVHeads, g.ropeFreqBase)
	}

	// d. Update KV cache.
	kv.Update(layer, k, v)

	// e. Compute grouped-query attention.
	attnOut := g.groupedQueryAttention(layer, q, kv)

	// f. Output projection.
	oWeight := g.weights[prefix+"attn_output.weight"].Data
	attnProj := MatVecMul(oWeight, attnOut, g.embedDim, g.embedDim)

	// g. Residual connection with scaling.
	residual := make([]float32, g.embedDim)
	if g.residualMultiplier != 1.0 {
		AddScaled(residual, x, attnProj, g.residualMultiplier)
	} else {
		Add(residual, x, attnProj)
	}

	return residual
}

// groupedQueryAttention computes multi-head attention with GQA support.
func (g *GraniteHybridArch) groupedQueryAttention(layer int, q []float32, kv *KVCache) []float32 {
	seqLen := len(kv.Keys[layer][0]) / g.headDim
	kvGroupSize := g.nHeads / g.nKVHeads
	scale := float32(1.0 / math.Sqrt(float64(g.headDim)))

	out := make([]float32, g.nHeads*g.headDim)

	for h := 0; h < g.nHeads; h++ {
		kvHead := h / kvGroupSize
		qVec := q[h*g.headDim : (h+1)*g.headDim]
		cachedK := kv.KeysForHead(layer, kvHead)
		cachedV := kv.ValuesForHead(layer, kvHead)

		scores := make([]float32, seqLen)
		for s := 0; s < seqLen; s++ {
			kVec := cachedK[s*g.headDim : (s+1)*g.headDim]
			var dot float32
			for d := 0; d < g.headDim; d++ {
				dot += qVec[d] * kVec[d]
			}
			scores[s] = dot * scale
		}

		Softmax(scores)

		headOut := out[h*g.headDim : (h+1)*g.headDim]
		for s := 0; s < seqLen; s++ {
			vVec := cachedV[s*g.headDim : (s+1)*g.headDim]
			w := scores[s]
			for d := 0; d < g.headDim; d++ {
				headOut[d] += w * vVec[d]
			}
		}
	}

	return out
}

// ffnBlock runs the SwiGLU feed-forward network for a layer.
func (g *GraniteHybridArch) ffnBlock(layer int, x []float32) []float32 {
	prefix := fmt.Sprintf("blk.%d.", layer)

	// Check if this layer has FFN weights (some hybrid configs may not).
	ffnNormW, ok := g.weights[prefix+"ffn_norm.weight"]
	if !ok {
		return x
	}

	ffnNormed := make([]float32, g.embedDim)
	RMSNorm(ffnNormed, x, ffnNormW.Data, g.rmsEps)

	gateWeight := g.weights[prefix+"ffn_gate.weight"].Data
	upWeight := g.weights[prefix+"ffn_up.weight"].Data
	downWeight := g.weights[prefix+"ffn_down.weight"].Data

	gate := MatVecMul(gateWeight, ffnNormed, g.ffDim, g.embedDim)
	up := MatVecMul(upWeight, ffnNormed, g.ffDim, g.embedDim)

	SiLU(gate)
	ElemMul(gate, gate, up)

	ffnOut := MatVecMul(downWeight, gate, g.embedDim, g.ffDim)

	out := make([]float32, g.embedDim)
	if g.residualMultiplier != 1.0 {
		AddScaled(out, x, ffnOut, g.residualMultiplier)
	} else {
		Add(out, x, ffnOut)
	}

	return out
}

// mambaBlock runs a Mamba2 SSM block for a single token decode step.
func (g *GraniteHybridArch) mambaBlock(layer int, x []float32, kv *KVCache) []float32 {
	prefix := fmt.Sprintf("blk.%d.", layer)
	mp := g.mamba

	// a. RMSNorm (attention norm — shared norm name).
	attnNormW := g.weights[prefix+"attn_norm.weight"].Data
	normed := make([]float32, g.embedDim)
	RMSNorm(normed, x, attnNormW, g.rmsEps)

	// b. Input projection: [projectionSize, embedDim] × [embedDim] → [projectionSize]
	wIn := g.weights[prefix+"ssm_in.weight"].Data
	projSize := mp.dInner + mp.convDim + mp.nHeads
	projected := MatVecMul(wIn, normed, projSize, g.embedDim)

	// c. Split: gate | hidden_B_C | dt
	gate := projected[:mp.dInner]
	hiddenBC := projected[mp.dInner : mp.dInner+mp.convDim]
	dt := projected[mp.dInner+mp.convDim:]

	// d. Depthwise causal conv1d over hiddenBC.
	wConv := g.weights[prefix+"ssm_conv1d.weight"].Data
	var bConv []float32
	if w, ok := g.weights[prefix+"ssm_conv1d.bias"]; ok {
		bConv = w.Data
	}
	convOut := Conv1DDepthwiseDecode(kv.Mamba.Layers[layer].ConvState, hiddenBC, wConv, bConv, mp.convDim, mp.kernel)

	// Apply SiLU activation to conv output.
	SiLU(convOut)

	// e. Split conv output: hidden | B | C
	groupsTimesState := mp.nGroups * mp.dState
	hidden := convOut[:mp.dInner]
	B := convOut[mp.dInner : mp.dInner+groupsTimesState]
	C := convOut[mp.dInner+groupsTimesState:]

	// f. SSM recurrence step.
	wA := g.weights[prefix+"ssm_a"].Data
	wD := g.weights[prefix+"ssm_d"].Data
	dtBias := g.weights[prefix+"ssm_dt.bias"].Data

	y := g.ssmStep(layer, hidden, B, C, dt, wA, wD, dtBias, kv)

	// g. Gated RMSNorm: norm(y, gate).
	normW := g.weights[prefix+"ssm_norm.weight"].Data
	yNormed := make([]float32, mp.dInner)
	RMSNormGated(yNormed, y, gate, normW, g.rmsEps)

	// h. Output projection.
	wOut := g.weights[prefix+"ssm_out.weight"].Data
	mambaOut := MatVecMul(wOut, yNormed, g.embedDim, mp.dInner)

	// i. Residual connection with scaling.
	out := make([]float32, g.embedDim)
	if g.residualMultiplier != 1.0 {
		AddScaled(out, x, mambaOut, g.residualMultiplier)
	} else {
		Add(out, x, mambaOut)
	}

	return out
}

// ssmStep performs a single Mamba2 SSM recurrence step.
// hidden: [dInner], B: [nGroups*dState], C: [nGroups*dState], dt: [nHeads]
// Returns y: [dInner].
func (g *GraniteHybridArch) ssmStep(layer int, hidden, B, C, dt, A, D, dtBias []float32, kv *KVCache) []float32 {
	mp := g.mamba
	ssmState := kv.Mamba.Layers[layer].SSMState
	groupDim := mp.dInner / mp.nGroups

	// Precompute dt values per head: softplus(dt + dt_bias).
	dtHead := make([]float32, mp.nHeads)
	for h := 0; h < mp.nHeads; h++ {
		dtHead[h] = Softplus(dt[h] + dtBias[h])
	}

	y := make([]float32, mp.dInner)

	for c := 0; c < mp.dInner; c++ {
		h := c / mp.innerHeadDim
		grp := c / groupDim
		dtv := dtHead[h]

		var yc float32
		for s := 0; s < mp.dState; s++ {
			// A values: In GGUF, A is stored as -exp(A_log) (pre-negated).
			// So A[h*dState+s] is already negative. dA = exp(dt * A).
			aIdx := h*mp.dState + s
			aVal := A[aIdx]
			dA := float32(math.Exp(float64(dtv * aVal)))

			stateIdx := c*mp.dState + s
			bIdx := grp*mp.dState + s

			// State update: ssm_state = dA * ssm_state + dt * hidden * B
			ssmState[stateIdx] = dA*ssmState[stateIdx] + dtv*hidden[c]*B[bIdx]

			// Output: y += state * C
			cIdx := grp*mp.dState + s
			yc += ssmState[stateIdx] * C[cIdx]
		}

		// D residual: y += D[h] * hidden[c]
		yc += D[h] * hidden[c]

		y[c] = yc
	}

	return y
}
