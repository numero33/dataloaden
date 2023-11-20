package dataloaden

import "context"

// The Cache interface. If a custom cache is provided, it must implement this interface.
type Cache[K comparable, V any] interface {
	Get(context.Context, K) (*V, bool)
	Set(context.Context, K, V)
	Delete(context.Context, K) bool
	Clear(context.Context)
}

// InMemoryCache implements Cache interface where.
type InMemoryCache[K comparable, V any] struct {
	cache map[K]V
}

// NewInMemoryCache constructs a new InMemoryCache
func NewInMemoryCache[K comparable, V any]() *InMemoryCache[K, V] {
	return &InMemoryCache[K, V]{
		cache: make(map[K]V),
	}
}

func (c *InMemoryCache[K, V]) Get(ctx context.Context, key K) (*V, bool) {
	if v, ok := c.cache[key]; ok {
		return &v, true
	}
	return nil, false
}

func (c *InMemoryCache[K, V]) Set(ctx context.Context, key K, value V) {
	c.cache[key] = value
}

func (c *InMemoryCache[K, V]) Delete(ctx context.Context, key K) bool {
	if _, ok := c.cache[key]; ok {
		delete(c.cache, key)
		return true
	}
	return false
}

func (c *InMemoryCache[K, V]) Clear(ctx context.Context) {
	c.cache = make(map[K]V)
}
