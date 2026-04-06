package ml

import (
	"encoding/binary"
	"math"
	"testing"
)

// float32ToFloat16 converts float32 to float16 (for test data construction).
func float32ToFloat16(f float32) uint16 {
	b := math.Float32bits(f)
	sign := (b >> 16) & 0x8000
	exp := int((b>>23)&0xff) - 127
	mant := b & 0x7fffff

	switch {
	case exp > 15:
		return uint16(sign | 0x7c00) // Inf
	case exp < -14:
		return uint16(sign) // Zero/subnormal
	default:
		return uint16(sign | uint32(exp+15)<<10 | (mant >> 13))
	}
}

func putFloat16(buf []byte, f float32) {
	binary.LittleEndian.PutUint16(buf, float32ToFloat16(f))
}

func TestDequantF32(t *testing.T) {
	values := []float32{0.0, 1.0, -1.0, 3.14, -2.718}
	data := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	out, err := Dequantize(data, "F32", len(values))
	if err != nil {
		t.Fatalf("Dequantize F32: %v", err)
	}
	for i, v := range values {
		if out[i] != v {
			t.Errorf("F32[%d]: got %f, want %f", i, out[i], v)
		}
	}
}

func TestDequantF16(t *testing.T) {
	values := []float32{0.0, 1.0, -1.0, 0.5, -0.5}
	data := make([]byte, len(values)*2)
	for i, v := range values {
		putFloat16(data[i*2:], v)
	}
	out, err := Dequantize(data, "F16", len(values))
	if err != nil {
		t.Fatalf("Dequantize F16: %v", err)
	}
	for i, v := range values {
		diff := math.Abs(float64(out[i] - v))
		if diff > 0.001 {
			t.Errorf("F16[%d]: got %f, want %f (diff=%e)", i, out[i], v, diff)
		}
	}
}

func TestDequantF16SpecialValues(t *testing.T) {
	data := make([]byte, 6)
	// +0
	binary.LittleEndian.PutUint16(data[0:], 0x0000)
	// +Inf
	binary.LittleEndian.PutUint16(data[2:], 0x7c00)
	// -0
	binary.LittleEndian.PutUint16(data[4:], 0x8000)

	out, err := Dequantize(data, "F16", 3)
	if err != nil {
		t.Fatalf("Dequantize F16: %v", err)
	}
	if out[0] != 0.0 {
		t.Errorf("F16 +0: got %f", out[0])
	}
	if !math.IsInf(float64(out[1]), 1) {
		t.Errorf("F16 +Inf: got %f", out[1])
	}
	if out[2] != 0.0 || !math.Signbit(float64(out[2])) {
		t.Errorf("F16 -0: got %v", out[2])
	}
}

func TestDequantQ8_0(t *testing.T) {
	// One block: 32 elements, scale=2.0
	const blockSize = 32
	block := make([]byte, 34)
	putFloat16(block, 2.0)
	for j := 0; j < blockSize; j++ {
		block[2+j] = byte(int8(j - 16)) // values from -16 to 15
	}

	out, err := Dequantize(block, "Q8_0", blockSize)
	if err != nil {
		t.Fatalf("Dequantize Q8_0: %v", err)
	}
	scale := float16ToFloat32(float32ToFloat16(2.0))
	for j := 0; j < blockSize; j++ {
		expected := scale * float32(j-16)
		if math.Abs(float64(out[j]-expected)) > 0.01 {
			t.Errorf("Q8_0[%d]: got %f, want %f", j, out[j], expected)
		}
	}
}

func TestDequantQ4_0(t *testing.T) {
	// One block: 32 elements, scale=1.0
	const blockSize = 32
	block := make([]byte, 18)
	putFloat16(block, 1.0)
	// Pack nibbles: low nibble first, then high nibble
	// Values: low nibble for elements 0..15, high nibble for elements 16..31
	for j := 0; j < 16; j++ {
		lo := byte(j)      // will be (j - 8) after dequant
		hi := byte(15 - j) // will be (15-j - 8) = (7-j) after dequant
		block[2+j] = (hi << 4) | lo
	}

	out, err := Dequantize(block, "Q4_0", blockSize)
	if err != nil {
		t.Fatalf("Dequantize Q4_0: %v", err)
	}
	scale := float16ToFloat32(float32ToFloat16(1.0))
	for j := 0; j < 16; j++ {
		expectedLo := scale * float32(j-8)
		expectedHi := scale * float32(7-j)
		if math.Abs(float64(out[j]-expectedLo)) > 0.01 {
			t.Errorf("Q4_0[%d] (lo): got %f, want %f", j, out[j], expectedLo)
		}
		if math.Abs(float64(out[16+j]-expectedHi)) > 0.01 {
			t.Errorf("Q4_0[%d] (hi): got %f, want %f", 16+j, out[16+j], expectedHi)
		}
	}
}

func TestDequantQ4_1(t *testing.T) {
	// One block: 32 elements, scale=0.5, min=1.0
	const blockSize = 32
	block := make([]byte, 20)
	putFloat16(block, 0.5)     // d
	putFloat16(block[2:], 1.0) // m
	for j := 0; j < 16; j++ {
		lo := byte(j)
		hi := byte(15 - j)
		block[4+j] = (hi << 4) | lo
	}

	out, err := Dequantize(block, "Q4_1", blockSize)
	if err != nil {
		t.Fatalf("Dequantize Q4_1: %v", err)
	}
	scale := float16ToFloat32(float32ToFloat16(0.5))
	m := float16ToFloat32(float32ToFloat16(1.0))
	for j := 0; j < 16; j++ {
		expectedLo := scale*float32(j) + m
		expectedHi := scale*float32(15-j) + m
		if math.Abs(float64(out[j]-expectedLo)) > 0.01 {
			t.Errorf("Q4_1[%d] (lo): got %f, want %f", j, out[j], expectedLo)
		}
		if math.Abs(float64(out[16+j]-expectedHi)) > 0.01 {
			t.Errorf("Q4_1[%d] (hi): got %f, want %f", 16+j, out[16+j], expectedHi)
		}
	}
}

func TestDequantQ4K(t *testing.T) {
	const qk = 256
	const blockBytes = 144
	block := make([]byte, blockBytes)

	// d=1.0, dmin=0.0 (simple case)
	putFloat16(block, 1.0)
	putFloat16(block[2:], 0.0)

	// Set all 8 sub-block scales to 1 and mins to 0.
	// getScaleMinK4 for j<4: sc = scales[j] & 63, m = scales[j+4] & 63
	// getScaleMinK4 for j>=4: sc = (scales[j+4] & 0xF) | ((scales[j-4] >> 6) << 4)
	//                         m  = (scales[j+4] >> 4)   | ((scales[j]   >> 6) << 4)
	// For j=0..3: set scales[j]=1 → sc=1, scales[j+4]=0 → m=0
	// For j=4..7: need (scales[j+4] & 0xF)=1 and (scales[j-4] >> 6)=0 → sc=1
	//             need (scales[j+4] >> 4)=0  and (scales[j] >> 6)=0   → m=0
	scales := block[4 : 4+12]
	for j := 0; j < 4; j++ {
		scales[j] = 1 // covers j<4: sc=1
	}
	for j := 8; j < 12; j++ {
		scales[j] = 1 // covers j>=4: (scales[j+4] & 0xF)=1 where j+4 maps to scales[8..11]
	}

	// Fill quants: all nibbles = 5
	q := block[4+12:]
	for j := range q {
		q[j] = 0x55 // lo=5, hi=5
	}

	out, err := Dequantize(block, "Q4_K", qk)
	if err != nil {
		t.Fatalf("Dequantize Q4_K: %v", err)
	}
	d := float16ToFloat32(float32ToFloat16(1.0))
	expected := d * 1.0 * 5.0 // d * sc * nibble - dmin * m = 1*1*5 - 0 = 5
	for j := 0; j < qk; j++ {
		if math.Abs(float64(out[j]-expected)) > 0.01 {
			t.Errorf("Q4_K[%d]: got %f, want %f", j, out[j], expected)
			break
		}
	}
}

func TestDequantQ6K(t *testing.T) {
	const qk = 256
	const blockBytes = 210
	block := make([]byte, blockBytes)

	// Layout: ql[128] + qh[64] + scales[16] + d[2]
	// Set d=1.0
	binary.LittleEndian.PutUint16(block[128+64+16:], float32ToFloat16(1.0))

	// Set all scales to 1
	scalesOff := 128 + 64
	for j := 0; j < 16; j++ {
		block[scalesOff+j] = 1
	}

	// Set ql all zero, qh all zero → q = 0 - 32 = -32
	// value = d * scale * q = 1 * 1 * (-32) = -32
	out, err := Dequantize(block, "Q6_K", qk)
	if err != nil {
		t.Fatalf("Dequantize Q6_K: %v", err)
	}

	d := float16ToFloat32(float32ToFloat16(1.0))
	expected := d * 1.0 * (-32.0)
	for j := 0; j < qk; j++ {
		if math.Abs(float64(out[j]-expected)) > 0.01 {
			t.Errorf("Q6_K[%d]: got %f, want %f", j, out[j], expected)
			break
		}
	}
}

func TestDequantQ5K(t *testing.T) {
	const qk = 256
	const blockBytes = 176
	block := make([]byte, blockBytes)

	// Layout: d[2] + dmin[2] + scales[12] + qh[32] + qs[128]
	putFloat16(block, 1.0)   // d
	putFloat16(block[2:], 0) // dmin

	// Set scale for sub-block 0 to 1, all mins to 0
	scales := block[4 : 4+12]
	for j := 0; j < 4; j++ {
		scales[j] = 1
	}

	// qs = all zeros, qh = all zeros → value = d * sc * (0 + 0) - 0 = 0
	out, err := Dequantize(block, "Q5_K", qk)
	if err != nil {
		t.Fatalf("Dequantize Q5_K: %v", err)
	}
	for j := 0; j < qk; j++ {
		if out[j] != 0 {
			t.Errorf("Q5_K[%d]: got %f, want 0", j, out[j])
			break
		}
	}
}

func TestDequantUnsupported(t *testing.T) {
	_, err := Dequantize(nil, "UNKNOWN", 0)
	if err == nil {
		t.Error("expected error for unsupported type")
	}
}

func TestDequantInsufficientData(t *testing.T) {
	tests := []struct {
		name  string
		qtype string
		n     int
		data  int // bytes to provide
	}{
		{"F32", "F32", 4, 8},     // need 16
		{"F16", "F16", 4, 4},     // need 8
		{"Q8_0", "Q8_0", 32, 10}, // need 34
		{"Q4_0", "Q4_0", 32, 10}, // need 18
		{"Q4_1", "Q4_1", 32, 10}, // need 20
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Dequantize(make([]byte, tt.data), tt.qtype, tt.n)
			if err == nil {
				t.Error("expected error for insufficient data")
			}
		})
	}
}
