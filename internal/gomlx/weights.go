package gomlx

import (
	"fmt"
	"os"

	gguf "github.com/gpustack/gguf-parser-go"
)

// WeightMap holds dequantized model weights indexed by tensor name.
type WeightMap map[string]*WeightTensor

// WeightTensor holds a dequantized tensor with its shape.
type WeightTensor struct {
	Name  string
	Shape []int
	Data  []float32
}

// LoadWeights loads and dequantizes all tensors from a GGUF file.
func LoadWeights(path string) (WeightMap, error) {
	gf, err := gguf.ParseGGUFFile(path, gguf.SkipLargeMetadata())
	if err != nil {
		return nil, fmt.Errorf("parse GGUF file: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open GGUF file: %w", err)
	}
	defer f.Close()

	dataStart := gf.TensorDataStartOffset
	wm := make(WeightMap, len(gf.TensorInfos))

	for _, ti := range gf.TensorInfos {
		numElements := int(ti.Elements())
		rawBytes := int(ti.Bytes())
		quantType := ti.Type.String()

		buf := make([]byte, rawBytes)
		offset := dataStart + int64(ti.Offset)
		if _, err := f.ReadAt(buf, offset); err != nil {
			return nil, fmt.Errorf("read tensor %q at offset %d: %w", ti.Name, offset, err)
		}

		data, err := Dequantize(buf, quantType, numElements)
		if err != nil {
			return nil, fmt.Errorf("dequantize tensor %q (type %s): %w", ti.Name, quantType, err)
		}

		shape := make([]int, len(ti.Dimensions))
		for d, dim := range ti.Dimensions {
			shape[d] = int(dim)
		}

		wm[ti.Name] = &WeightTensor{
			Name:  ti.Name,
			Shape: shape,
			Data:  data,
		}
	}

	return wm, nil
}
