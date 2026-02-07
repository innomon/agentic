package compreg

import "sync"

var (
	mu    sync.RWMutex
	items = make(map[string]any)
)

func Set(key string, val any) {
	mu.Lock()
	defer mu.Unlock()
	items[key] = val
}

func Lookup[T any](key string) (T, bool) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := items[key]
	if !ok {
		var zero T
		return zero, false
	}
	t, ok := v.(T)
	return t, ok
}
