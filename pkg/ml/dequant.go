package ml

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Dequantize converts quantized GGUF tensor data to float32 values.
// quantType is the GGUF type string (e.g., "F32", "F16", "Q8_0", "Q4_0", etc.).
func Dequantize(data []byte, quantType string, numElements int) ([]float32, error) {
	switch quantType {
	case "F32":
		return dequantF32(data, numElements)
	case "F16":
		return dequantF16(data, numElements)
	case "Q8_0":
		return dequantQ8_0(data, numElements)
	case "Q4_0":
		return dequantQ4_0(data, numElements)
	case "Q4_1":
		return dequantQ4_1(data, numElements)
	case "Q4_K":
		return dequantQ4K(data, numElements)
	case "Q5_K":
		return dequantQ5K(data, numElements)
	case "Q6_K":
		return dequantQ6K(data, numElements)
	default:
		return nil, fmt.Errorf("unsupported quantization type: %s", quantType)
	}
}

// float16ToFloat32 converts a half-precision float (uint16) to float32.
func float16ToFloat32(h uint16) float32 {
	sign := uint32(h>>15) & 1
	exp := uint32(h>>10) & 0x1f
	mant := uint32(h) & 0x3ff

	switch {
	case exp == 0:
		if mant == 0 {
			return math.Float32frombits(sign << 31)
		}
		// Subnormal: normalize
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		exp++
		mant &= 0x3ff
		return math.Float32frombits((sign << 31) | ((exp + 112) << 23) | (mant << 13))
	case exp == 0x1f:
		// Inf or NaN
		return math.Float32frombits((sign << 31) | 0x7f800000 | (mant << 13))
	default:
		return math.Float32frombits((sign << 31) | ((exp + 112) << 23) | (mant << 13))
	}
}

// dequantF32 reinterprets raw bytes as little-endian float32 values.
func dequantF32(data []byte, n int) ([]float32, error) {
	need := n * 4
	if len(data) < need {
		return nil, fmt.Errorf("F32: need %d bytes, got %d", need, len(data))
	}
	out := make([]float32, n)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return out, nil
}

// dequantF16 converts half-precision floats to float32.
func dequantF16(data []byte, n int) ([]float32, error) {
	need := n * 2
	if len(data) < need {
		return nil, fmt.Errorf("F16: need %d bytes, got %d", need, len(data))
	}
	out := make([]float32, n)
	for i := range out {
		h := binary.LittleEndian.Uint16(data[i*2:])
		out[i] = float16ToFloat32(h)
	}
	return out, nil
}

// dequantQ8_0 dequantizes Q8_0 blocks.
// Block layout: float16 scale (2 bytes) + 32 int8 quants (32 bytes) = 34 bytes.
func dequantQ8_0(data []byte, n int) ([]float32, error) {
	const blockSize = 32
	const blockBytes = 2 + blockSize // 34

	if n%blockSize != 0 {
		return nil, fmt.Errorf("Q8_0: element count %d not a multiple of %d", n, blockSize)
	}
	nBlocks := n / blockSize
	need := nBlocks * blockBytes
	if len(data) < need {
		return nil, fmt.Errorf("Q8_0: need %d bytes, got %d", need, len(data))
	}

	out := make([]float32, n)
	for i := 0; i < nBlocks; i++ {
		block := data[i*blockBytes:]
		d := float16ToFloat32(binary.LittleEndian.Uint16(block))
		for j := 0; j < blockSize; j++ {
			out[i*blockSize+j] = d * float32(int8(block[2+j]))
		}
	}
	return out, nil
}

// dequantQ4_0 dequantizes Q4_0 blocks.
// Block layout: float16 scale (2 bytes) + 16 bytes (32 nibbles) = 18 bytes.
// Nibbles are subtracted by 8 to center around 0.
func dequantQ4_0(data []byte, n int) ([]float32, error) {
	const blockSize = 32
	const blockBytes = 2 + blockSize/2 // 18

	if n%blockSize != 0 {
		return nil, fmt.Errorf("Q4_0: element count %d not a multiple of %d", n, blockSize)
	}
	nBlocks := n / blockSize
	need := nBlocks * blockBytes
	if len(data) < need {
		return nil, fmt.Errorf("Q4_0: need %d bytes, got %d", need, len(data))
	}

	out := make([]float32, n)
	for i := 0; i < nBlocks; i++ {
		block := data[i*blockBytes:]
		d := float16ToFloat32(binary.LittleEndian.Uint16(block))
		for j := 0; j < blockSize/2; j++ {
			b := block[2+j]
			lo := int(b&0x0F) - 8
			hi := int(b>>4) - 8
			out[i*blockSize+j] = d * float32(lo)
			out[i*blockSize+j+blockSize/2] = d * float32(hi)
		}
	}
	return out, nil
}

// dequantQ4_1 dequantizes Q4_1 blocks.
// Block layout: float16 scale (2 bytes) + float16 min (2 bytes) + 16 bytes = 20 bytes.
func dequantQ4_1(data []byte, n int) ([]float32, error) {
	const blockSize = 32
	const blockBytes = 4 + blockSize/2 // 20

	if n%blockSize != 0 {
		return nil, fmt.Errorf("Q4_1: element count %d not a multiple of %d", n, blockSize)
	}
	nBlocks := n / blockSize
	need := nBlocks * blockBytes
	if len(data) < need {
		return nil, fmt.Errorf("Q4_1: need %d bytes, got %d", need, len(data))
	}

	out := make([]float32, n)
	for i := 0; i < nBlocks; i++ {
		block := data[i*blockBytes:]
		d := float16ToFloat32(binary.LittleEndian.Uint16(block))
		m := float16ToFloat32(binary.LittleEndian.Uint16(block[2:]))
		for j := 0; j < blockSize/2; j++ {
			b := block[4+j]
			lo := int(b & 0x0F)
			hi := int(b >> 4)
			out[i*blockSize+j] = d*float32(lo) + m
			out[i*blockSize+j+blockSize/2] = d*float32(hi) + m
		}
	}
	return out, nil
}

// getScaleMinK4 unpacks scale and min values from the 12-byte packed format
// used by Q4_K and Q5_K. Matches llama.cpp get_scale_min_k4.
func getScaleMinK4(j int, scales []byte) (sc, m uint8) {
	if j < 4 {
		sc = scales[j] & 63
		m = scales[j+4] & 63
	} else {
		sc = (scales[j+4] & 0x0F) | ((scales[j-4] >> 6) << 4)
		m = (scales[j+4] >> 4) | ((scales[j] >> 6) << 4)
	}
	return
}

// dequantQ4K dequantizes Q4_K (k-quant 4-bit) super-blocks.
// Super-block (256 elements): float16 d (2) + float16 dmin (2) + 12 byte scales + 128 byte quants = 144 bytes.
func dequantQ4K(data []byte, n int) ([]float32, error) {
	const qk = 256
	const blockBytes = 2 + 2 + 12 + qk/2 // 144

	if n%qk != 0 {
		return nil, fmt.Errorf("Q4_K: element count %d not a multiple of %d", n, qk)
	}
	nBlocks := n / qk
	need := nBlocks * blockBytes
	if len(data) < need {
		return nil, fmt.Errorf("Q4_K: need %d bytes, got %d", need, len(data))
	}

	out := make([]float32, n)
	for i := 0; i < nBlocks; i++ {
		block := data[i*blockBytes:]
		d := float16ToFloat32(binary.LittleEndian.Uint16(block))
		dmin := float16ToFloat32(binary.LittleEndian.Uint16(block[2:]))
		scales := block[4 : 4+12]
		q := block[4+12:]

		is := 0
		outOff := i * qk
		qOff := 0
		for j := 0; j < qk; j += 64 {
			sc1, m1 := getScaleMinK4(is+0, scales)
			d1 := d * float32(sc1)
			m1f := dmin * float32(m1)
			sc2, m2 := getScaleMinK4(is+1, scales)
			d2 := d * float32(sc2)
			m2f := dmin * float32(m2)

			for l := 0; l < 32; l++ {
				out[outOff+j+l] = d1*float32(q[qOff+l]&0x0F) - m1f
			}
			for l := 0; l < 32; l++ {
				out[outOff+j+32+l] = d2*float32(q[qOff+l]>>4) - m2f
			}
			qOff += 32
			is += 2
		}
	}
	return out, nil
}

// dequantQ5K dequantizes Q5_K (k-quant 5-bit) super-blocks.
// Super-block (256 elements): float16 d (2) + float16 dmin (2) + 12 byte scales + 32 byte qh + 128 byte qs = 176 bytes.
func dequantQ5K(data []byte, n int) ([]float32, error) {
	const qk = 256
	const blockBytes = 2 + 2 + 12 + qk/8 + qk/2 // 176

	if n%qk != 0 {
		return nil, fmt.Errorf("Q5_K: element count %d not a multiple of %d", n, qk)
	}
	nBlocks := n / qk
	need := nBlocks * blockBytes
	if len(data) < need {
		return nil, fmt.Errorf("Q5_K: need %d bytes, got %d", need, len(data))
	}

	out := make([]float32, n)
	for i := 0; i < nBlocks; i++ {
		block := data[i*blockBytes:]
		d := float16ToFloat32(binary.LittleEndian.Uint16(block))
		dmin := float16ToFloat32(binary.LittleEndian.Uint16(block[2:]))
		scales := block[4 : 4+12]
		qh := block[4+12 : 4+12+qk/8]
		ql := block[4+12+qk/8:]

		is := 0
		outOff := i * qk
		u1 := uint8(1)
		u2 := uint8(2)
		qlOff := 0
		for j := 0; j < qk; j += 64 {
			sc1, m1 := getScaleMinK4(is+0, scales)
			d1 := d * float32(sc1)
			m1f := dmin * float32(m1)
			sc2, m2 := getScaleMinK4(is+1, scales)
			d2 := d * float32(sc2)
			m2f := dmin * float32(m2)

			for l := 0; l < 32; l++ {
				hiBit := uint8(0)
				if qh[l]&u1 != 0 {
					hiBit = 16
				}
				out[outOff+j+l] = d1*float32(uint8(ql[qlOff+l]&0x0F)+hiBit) - m1f
			}
			for l := 0; l < 32; l++ {
				hiBit := uint8(0)
				if qh[l]&u2 != 0 {
					hiBit = 16
				}
				out[outOff+j+32+l] = d2*float32(uint8(ql[qlOff+l]>>4)+hiBit) - m2f
			}
			qlOff += 32
			is += 2
			u1 <<= 2
			u2 <<= 2
		}
	}
	return out, nil
}

// dequantQ6K dequantizes Q6_K (k-quant 6-bit) super-blocks.
// Super-block (256 elements): 128 byte ql + 64 byte qh + 16 byte scales + float16 d (2) = 210 bytes.
func dequantQ6K(data []byte, n int) ([]float32, error) {
	const qk = 256
	const blockBytes = qk/2 + qk/4 + qk/16 + 2 // 128 + 64 + 16 + 2 = 210

	if n%qk != 0 {
		return nil, fmt.Errorf("Q6_K: element count %d not a multiple of %d", n, qk)
	}
	nBlocks := n / qk
	need := nBlocks * blockBytes
	if len(data) < need {
		return nil, fmt.Errorf("Q6_K: need %d bytes, got %d", need, len(data))
	}

	out := make([]float32, n)
	for i := 0; i < nBlocks; i++ {
		block := data[i*blockBytes:]
		ql := block[:qk/2]         // 128 bytes
		qh := block[qk/2 : 3*qk/4] // 64 bytes
		sc := block[3*qk/4 : 3*qk/4+qk/16]
		d := float16ToFloat32(binary.LittleEndian.Uint16(block[3*qk/4+qk/16:]))

		outOff := i * qk
		qlOff := 0
		qhOff := 0
		scOff := 0

		for blk := 0; blk < qk; blk += 128 {
			for l := 0; l < 32; l++ {
				is := l / 16
				q1 := int8((ql[qlOff+l]&0x0F)|((qh[qhOff+l]>>0)&3)<<4) - 32
				q2 := int8((ql[qlOff+l+32]&0x0F)|((qh[qhOff+l]>>2)&3)<<4) - 32
				q3 := int8((ql[qlOff+l]>>4)|((qh[qhOff+l]>>4)&3)<<4) - 32
				q4 := int8((ql[qlOff+l+32]>>4)|((qh[qhOff+l]>>6)&3)<<4) - 32

				out[outOff+blk+l] = d * float32(int8(sc[scOff+is])) * float32(q1)
				out[outOff+blk+l+32] = d * float32(int8(sc[scOff+is+2])) * float32(q2)
				out[outOff+blk+l+64] = d * float32(int8(sc[scOff+is+4])) * float32(q3)
				out[outOff+blk+l+96] = d * float32(int8(sc[scOff+is+6])) * float32(q4)
			}
			qlOff += 64
			qhOff += 32
			scOff += 8
		}
	}
	return out, nil
}
