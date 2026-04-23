package ml

import (
	"math"
	"runtime"
	"sync"
)

var (
	numThreads = runtime.NumCPU()
)

// SetThreads sets the number of threads to use for parallel operations.
// If n <= 0, it defaults to runtime.NumCPU().
func SetThreads(n int) {
	if n <= 0 {
		numThreads = runtime.NumCPU()
	} else {
		numThreads = n
	}
}

// MatMul computes C = A × B where A is [m,k] (row-major) and B is [k,n] (row-major).
// Returns C of shape [m,n].
func MatMul(a, b []float32, m, k, n int) []float32 {
	c := make([]float32, m*n)
	nThreads := numThreads
	if m < nThreads {
		nThreads = m
	}
	if nThreads <= 1 {
		for i := 0; i < m; i++ {
			aRow := a[i*k : i*k+k]
			cRow := c[i*n : i*n+n]
			for p := 0; p < k; p++ {
				ap := aRow[p]
				bRow := b[p*n : p*n+n]
				for j := 0; j < n; j++ {
					cRow[j] += ap * bRow[j]
				}
			}
		}
		return c
	}

	var wg sync.WaitGroup
	chunkSize := (m + nThreads - 1) / nThreads
	for t := 0; t < nThreads; t++ {
		start := t * chunkSize
		end := start + chunkSize
		if start >= m {
			break
		}
		if end > m {
			end = m
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				aRow := a[i*k : i*k+k]
				cRow := c[i*n : i*n+n]
				for p := 0; p < k; p++ {
					ap := aRow[p]
					bRow := b[p*n : p*n+n]
					for j := 0; j < n; j++ {
						cRow[j] += ap * bRow[j]
					}
				}
			}
		}(start, end)
	}
	wg.Wait()
	return c
}

// MatVecMul computes y = A × x where A is [m,k] (row-major) and x is [k].
// Returns y of length m.
func MatVecMul(a, x []float32, m, k int) []float32 {
	y := make([]float32, m)
	nThreads := numThreads
	if m < nThreads {
		nThreads = m
	}
	// Heuristic: don't parallelize if work is too small.
	if nThreads <= 1 || m*k < 1024 {
		for i := 0; i < m; i++ {
			row := a[i*k : i*k+k]
			var sum float32
			for j := 0; j < k; j++ {
				sum += row[j] * x[j]
			}
			y[i] = sum
		}
		return y
	}

	var wg sync.WaitGroup
	chunkSize := (m + nThreads - 1) / nThreads
	for t := 0; t < nThreads; t++ {
		start := t * chunkSize
		end := start + chunkSize
		if start >= m {
			break
		}
		if end > m {
			end = m
		}
		wg.Add(1)
		go func(s, e int) {
			defer wg.Done()
			for i := s; i < e; i++ {
				row := a[i*k : i*k+k]
				var sum float32
				for j := 0; j < k; j++ {
					sum += row[j] * x[j]
				}
				y[i] = sum
			}
		}(start, end)
	}
	wg.Wait()
	return y
}

// RMSNorm computes RMS normalization: out_i = x_i * weight_i / sqrt(mean(x^2) + eps).
// out, x, and weight must all have the same length.
func RMSNorm(out, x, weight []float32, eps float32) {
	n := len(x)
	var ss float32
	for i := 0; i < n; i++ {
		ss += x[i] * x[i]
	}
	ss = ss/float32(n) + eps
	rs := float32(1.0 / math.Sqrt(float64(ss)))
	for i := 0; i < n; i++ {
		out[i] = x[i] * rs * weight[i]
	}
}

// Softmax computes softmax in-place over x.
func Softmax(x []float32) {
	n := len(x)
	if n == 0 {
		return
	}
	// Find max for numerical stability.
	maxVal := x[0]
	for i := 1; i < n; i++ {
		if x[i] > maxVal {
			maxVal = x[i]
		}
	}
	var sum float32
	for i := 0; i < n; i++ {
		x[i] = float32(math.Exp(float64(x[i] - maxVal)))
		sum += x[i]
	}
	inv := 1.0 / sum
	for i := 0; i < n; i++ {
		x[i] *= inv
	}
}

// SiLU computes the SiLU (Swish) activation in-place: x_i = x_i * sigmoid(x_i).
func SiLU(x []float32) {
	for i := range x {
		x[i] = x[i] * float32(1.0/(1.0+math.Exp(float64(-x[i]))))
	}
}

// RoPE applies Rotary Position Embeddings to query and key vectors.
// q has nHeads*headDim elements, k has nKVHeads*headDim elements.
// pos is the token position, freqBase is the RoPE frequency base (e.g., 10000).
func RoPE(q, k []float32, headDim, pos, nHeads, nKVHeads int, freqBase float32) {
	halfDim := headDim / 2
	for h := 0; h < nHeads; h++ {
		qOff := h * headDim
		for i := 0; i < halfDim; i++ {
			freq := 1.0 / float32(math.Pow(float64(freqBase), float64(2*i)/float64(headDim)))
			theta := float32(pos) * freq
			cos := float32(math.Cos(float64(theta)))
			sin := float32(math.Sin(float64(theta)))
			q0 := q[qOff+2*i]
			q1 := q[qOff+2*i+1]
			q[qOff+2*i] = q0*cos - q1*sin
			q[qOff+2*i+1] = q0*sin + q1*cos
		}
	}
	for h := 0; h < nKVHeads; h++ {
		kOff := h * headDim
		for i := 0; i < halfDim; i++ {
			freq := 1.0 / float32(math.Pow(float64(freqBase), float64(2*i)/float64(headDim)))
			theta := float32(pos) * freq
			cos := float32(math.Cos(float64(theta)))
			sin := float32(math.Sin(float64(theta)))
			k0 := k[kOff+2*i]
			k1 := k[kOff+2*i+1]
			k[kOff+2*i] = k0*cos - k1*sin
			k[kOff+2*i+1] = k0*sin + k1*cos
		}
	}
}

// Add computes out = a + b element-wise. All slices must have the same length.
func Add(out, a, b []float32) {
	for i := range out {
		out[i] = a[i] + b[i]
	}
}

// ElemMul computes out = a * b element-wise. All slices must have the same length.
func ElemMul(out, a, b []float32) {
	for i := range out {
		out[i] = a[i] * b[i]
	}
}

// Softplus computes log(1 + exp(x)) with numerical stability.
func Softplus(x float32) float32 {
	if x > 20 {
		return x
	}
	return float32(math.Log1p(math.Exp(float64(x))))
}

// AddScaled computes out = a + scale*b element-wise. All slices must have the same length.
func AddScaled(out, a, b []float32, scale float32) {
	for i := range out {
		out[i] = a[i] + scale*b[i]
	}
}

// RMSNormGated computes gated RMS normalization: (x_i * weight_i / rms(x)) * silu(gate_i).
// out, x, gate, and weight must all have the same length.
func RMSNormGated(out, x, gate, weight []float32, eps float32) {
	n := len(x)
	var ss float32
	for i := 0; i < n; i++ {
		ss += x[i] * x[i]
	}
	ss = ss/float32(n) + eps
	rs := float32(1.0 / math.Sqrt(float64(ss)))
	for i := 0; i < n; i++ {
		g := gate[i] * float32(1.0/(1.0+math.Exp(float64(-gate[i])))) // SiLU(gate)
		out[i] = x[i] * rs * weight[i] * g
	}
}

// Conv1DDepthwiseDecode performs a single step of causal depthwise 1D convolution.
// convState is a shift register of shape [nChannels * (kernel-1)], updated in-place.
// input is the new input vector of shape [nChannels].
// weight has shape [nChannels * kernel] (depthwise: each channel has its own kernel).
// bias has shape [nChannels] (may be nil for no bias).
// Returns the output vector of shape [nChannels].
func Conv1DDepthwiseDecode(convState, input, weight, bias []float32, nChannels, kernel int) []float32 {
	out := make([]float32, nChannels)
	histLen := kernel - 1
	for c := 0; c < nChannels; c++ {
		stateOff := c * histLen
		// Shift state left: drop oldest, append new input
		for k := 0; k < histLen-1; k++ {
			convState[stateOff+k] = convState[stateOff+k+1]
		}
		convState[stateOff+histLen-1] = input[c]

		// Compute dot product of [history..., current] with kernel weights
		var sum float32
		wOff := c * kernel
		for k := 0; k < histLen; k++ {
			sum += convState[stateOff+k] * weight[wOff+k]
		}
		sum += input[c] * weight[wOff+histLen]
		if bias != nil {
			sum += bias[c]
		}
		out[c] = sum
	}
	return out
}
