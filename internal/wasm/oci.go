package wasm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/regclient/regclient"
	"github.com/regclient/regclient/types/descriptor"
	"github.com/regclient/regclient/types/manifest"
	"github.com/regclient/regclient/types/ref"
)

var wasmMediaTypes = []string{
	"application/wasm",
	"application/vnd.wasm.content.layer.v1+wasm",
}

type OCIPuller struct {
	rc       *regclient.RegClient
	cacheDir string
}

func NewOCIPuller(cacheDir string) *OCIPuller {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "med-agent-wasm-oci")
	}
	return &OCIPuller{
		rc:       regclient.New(),
		cacheDir: cacheDir,
	}
}

func (p *OCIPuller) PullWasm(ctx context.Context, ociRef string) ([]byte, error) {
	r, err := ref.New(ociRef)
	if err != nil {
		return nil, fmt.Errorf("invalid OCI reference %q: %w", ociRef, err)
	}

	m, err := p.rc.ManifestGet(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifest for %q: %w", ociRef, err)
	}

	mi, ok := m.(manifest.Imager)
	if !ok {
		return nil, fmt.Errorf("manifest for %q does not support layers", ociRef)
	}

	layers, err := mi.GetLayers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	for _, layer := range layers {
		mt := layer.MediaType
		if isWasmMediaType(mt) || hasWasmAnnotation(layer.Annotations) {
			return p.pullBlob(ctx, r, layer)
		}
	}

	if len(layers) > 0 {
		return p.pullBlob(ctx, r, layers[0])
	}

	return nil, fmt.Errorf("no wasm layer found in %q", ociRef)
}

func (p *OCIPuller) pullBlob(ctx context.Context, r ref.Ref, d descriptor.Descriptor) ([]byte, error) {
	digest := d.Digest.String()
	cached, err := p.fromCache(digest)
	if err == nil {
		return cached, nil
	}

	blob, err := p.rc.BlobGet(ctx, r, d)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob %s: %w", digest, err)
	}
	defer blob.Close()

	data, err := blob.RawBody()
	if err != nil {
		return nil, fmt.Errorf("failed to read blob body: %w", err)
	}

	_ = p.toCache(digest, data)

	return data, nil
}

func (p *OCIPuller) fromCache(digest string) ([]byte, error) {
	path := filepath.Join(p.cacheDir, sanitizeDigest(digest)+".wasm")
	return os.ReadFile(path)
}

func (p *OCIPuller) toCache(digest string, data []byte) error {
	if err := os.MkdirAll(p.cacheDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(p.cacheDir, sanitizeDigest(digest)+".wasm")
	return os.WriteFile(path, data, 0o644)
}

func sanitizeDigest(digest string) string {
	return strings.ReplaceAll(digest, ":", "-")
}

func isWasmMediaType(mt string) bool {
	for _, allowed := range wasmMediaTypes {
		if mt == allowed {
			return true
		}
	}
	return false
}

func hasWasmAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	for _, v := range annotations {
		if strings.HasSuffix(v, ".wasm") {
			return true
		}
	}
	return false
}
