package ml

import (
	"fmt"

	gguf "github.com/gpustack/gguf-parser-go"
)

// GGUFModelInfo holds extracted metadata and hyperparameters from a GGUF file.
type GGUFModelInfo struct {
	// Metadata
	Architecture       string `json:"architecture"`
	ModelName          string `json:"modelName,omitempty"`
	FileType           string `json:"fileType"`
	FileTypeDescriptor string `json:"fileTypeDescriptor"`

	// Hyperparameters
	EmbeddingLength    uint64   `json:"embeddingLength,omitempty"`
	BlockCount         uint64   `json:"blockCount,omitempty"`
	AttentionHeadCount uint64   `json:"attentionHeadCount,omitempty"`
	AttentionHeadKV    uint64   `json:"attentionHeadCountKV,omitempty"`
	FeedForwardLength  []uint64 `json:"feedForwardLength,omitempty"`
	VocabSize          uint64   `json:"vocabSize,omitempty"`
	ContextLength      uint64   `json:"contextLength,omitempty"`

	// Scaling factors (for Granite/MiniCPM-style architectures).
	EmbeddingScale float32 `json:"embeddingScale,omitempty"`
	ResidualScale  float32 `json:"residualScale,omitempty"`
	LogitScale     float32 `json:"logitScale,omitempty"`

	// Size estimates
	ModelParameters uint64  `json:"modelParameters"`
	ModelSize       uint64  `json:"modelSize"`
	FileSize        uint64  `json:"fileSize"`
	BitsPerWeight   float64 `json:"bitsPerWeight"`

	// Tensor summary
	TensorCount uint64           `json:"tensorCount"`
	Tensors     []GGUFTensorInfo `json:"tensors,omitempty"`
}

// GGUFTensorInfo holds basic information about a single tensor.
type GGUFTensorInfo struct {
	Name       string   `json:"name"`
	Dimensions []uint64 `json:"dimensions"`
	Type       string   `json:"type"`
}

// ParseGGUF parses a GGUF file at the given path and returns extracted model info.
func ParseGGUF(path string) (*GGUFModelInfo, error) {
	gf, err := gguf.ParseGGUFFile(path, gguf.SkipLargeMetadata())
	if err != nil {
		return nil, fmt.Errorf("parse GGUF file: %w", err)
	}

	meta := gf.Metadata()
	arch := gf.Architecture()

	info := &GGUFModelInfo{
		Architecture:       meta.Architecture,
		ModelName:          meta.Name,
		FileType:           meta.FileType.String(),
		FileTypeDescriptor: meta.FileTypeDescriptor,

		EmbeddingLength:    arch.EmbeddingLength,
		BlockCount:         arch.BlockCount,
		AttentionHeadCount: arch.AttentionHeadCount,
		AttentionHeadKV:    arch.AttentionHeadCountKV,
		FeedForwardLength:  arch.FeedForwardLength,
		VocabSize:          arch.VocabularyLength,
		ContextLength:      arch.MaximumContextLength,

		ModelParameters: uint64(gf.ModelParameters),
		ModelSize:       uint64(gf.ModelSize),
		FileSize:        uint64(gf.Size),
		BitsPerWeight:   float64(gf.ModelBitsPerWeight),

		TensorCount: gf.Header.TensorCount,
	}

	// Extract scaling factors from metadata (used by Granite, MiniCPM, etc.).
	archPrefix := meta.Architecture + "."
	info.EmbeddingScale = getMetadataFloat32(gf, archPrefix+"embedding_scale")
	info.ResidualScale = getMetadataFloat32(gf, archPrefix+"residual_scale")
	info.LogitScale = getMetadataFloat32(gf, archPrefix+"logit_scale")

	for _, ti := range gf.TensorInfos {
		info.Tensors = append(info.Tensors, GGUFTensorInfo{
			Name:       ti.Name,
			Dimensions: ti.Dimensions,
			Type:       ti.Type.String(),
		})
	}

	return info, nil
}

// getMetadataFloat32 safely extracts a float32 metadata value, returning 0 if not found.
func getMetadataFloat32(gf *gguf.GGUFFile, key string) float32 {
	kv, found := gf.Header.MetadataKV.Get(key)
	if !found {
		return 0
	}
	switch kv.ValueType {
	case gguf.GGUFMetadataValueTypeFloat32:
		return kv.ValueFloat32()
	case gguf.GGUFMetadataValueTypeFloat64:
		return float32(kv.ValueFloat64())
	default:
		return 0
	}
}

// EstimateMemoryBytes returns a rough estimate of memory needed to load the model.
// It uses the model size (weight data when loaded) plus a fixed overhead for runtime buffers.
func (info *GGUFModelInfo) EstimateMemoryBytes() uint64 {
	const runtimeOverhead = 256 * 1024 * 1024 // 256 MiB for KV cache and compute buffers
	return info.ModelSize + runtimeOverhead
}
