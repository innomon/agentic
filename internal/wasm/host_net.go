package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

type httpRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
}

type httpResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
	Error   string            `json:"error,omitempty"`
}

const maxHTTPResponseBody = 4 * 1024 * 1024 // 4MB

func registerNetHostFunctions(builder wazero.HostModuleBuilder, policy *SecurityPolicy) {
	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, reqPtr, reqLen int32) int64 {
			return hostHTTPFetch(ctx, mod, reqPtr, reqLen, policy)
		}).
		Export("http_fetch")
}

func hostHTTPFetch(ctx context.Context, mod api.Module, reqPtr, reqLen int32, policy *SecurityPolicy) int64 {
	reqBytes, ok := mod.Memory().Read(uint32(reqPtr), uint32(reqLen))
	if !ok {
		return writeErrorResponse(ctx, mod, "failed to read request from memory")
	}

	var req httpRequest
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("invalid request JSON: %v", err))
	}

	if err := policy.CheckDomain(req.URL); err != nil {
		return writeErrorResponse(ctx, mod, err.Error())
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = io.NopCloser(
			io.LimitReader(
				readerFromString(req.Body),
				maxHTTPResponseBody,
			),
		)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, bodyReader)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("failed to create request: %v", err))
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("HTTP request failed: %v", err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBody))
	if err != nil {
		return writeErrorResponse(ctx, mod, fmt.Sprintf("failed to read response body: %v", err))
	}

	headers := make(map[string]string)
	for k := range resp.Header {
		headers[k] = resp.Header.Get(k)
	}

	return writeResponse(ctx, mod, &httpResponse{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    string(body),
	})
}

func writeErrorResponse(ctx context.Context, mod api.Module, errMsg string) int64 {
	return writeResponse(ctx, mod, &httpResponse{Error: errMsg})
}

func writeResponse(ctx context.Context, mod api.Module, resp *httpResponse) int64 {
	data, err := json.Marshal(resp)
	if err != nil {
		return 0
	}

	alloc := mod.ExportedFunction("alloc")
	if alloc == nil {
		return 0
	}

	results, err := alloc.Call(ctx, uint64(len(data)))
	if err != nil || len(results) == 0 {
		return 0
	}

	ptr := uint32(results[0])
	if !mod.Memory().Write(ptr, data) {
		return 0
	}

	return int64(ptr)<<32 | int64(len(data))
}

type stringReader struct {
	s string
	i int
}

func readerFromString(s string) io.Reader {
	return &stringReader{s: s}
}

func (r *stringReader) Read(p []byte) (n int, err error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n = copy(p, r.s[r.i:])
	r.i += n
	return
}
