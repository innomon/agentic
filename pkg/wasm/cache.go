package wasm

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
)

var (
	compilationCacheOnce sync.Once
	compilationCache     wazero.CompilationCache
)

func getCompilationCache() wazero.CompilationCache {
	compilationCacheOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), "agentic-wasm-cache")
		cache, err := wazero.NewCompilationCacheWithDir(dir)
		if err != nil {
			cache = wazero.NewCompilationCache()
		}
		compilationCache = cache
	})
	return compilationCache
}

type bytecodeCache struct {
	mu    sync.RWMutex
	items map[string][]byte
}

var wasmBytecodeCache = &bytecodeCache{items: make(map[string][]byte)}

func (c *bytecodeCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	b, ok := c.items[key]
	return b, ok
}

func (c *bytecodeCache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = data
}
