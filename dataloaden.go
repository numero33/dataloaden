package dataloaden

import (
	"context"
	"sync"
	"time"
)

// BatchFunc is a function, which when given a slice of keys (string), returns a slice of `results`.
// It's important that the length of the input keys matches the length of the output results.
//
// The keys passed to this function are guaranteed to be unique
type BatchFunc[K comparable, V any] func(context.Context, []K) ([]V, []error)

// Option allows for configuration of Loader fields.
type Option[K comparable, V any] func(*Loader[K, V])

// WithCache sets the BatchedLoader cache. Defaults to InMemoryCache if a Cache is not set.
func WithCache[K comparable, V any](c Cache[K, V]) Option[K, V] {
	return func(l *Loader[K, V]) {
		l.cache = c
	}
}

// WithBatchCapacity sets the batch capacity. Default is 0 (unbounded).
func WithBatchCapacity[K comparable, V any](c int) Option[K, V] {
	return func(l *Loader[K, V]) {
		l.maxBatch = c
	}
}

// WithWait sets the amount of time to wait before triggering a batch.
// Default duration is 16 milliseconds.
func WithWait[K comparable, V any](d time.Duration) Option[K, V] {
	return func(l *Loader[K, V]) {
		l.wait = d
	}
}

// NewBatchLoader creates a new Loader given a fetch, wait, and maxBatch
func NewBatchLoader[K comparable, V any](batchFn BatchFunc[K, V], opts ...Option[K, V]) *Loader[K, V] {
	loader := &Loader[K, V]{
		fetch:    batchFn,
		wait:     16 * time.Millisecond,
		maxBatch: 0,
	}

	// Apply options
	for _, apply := range opts {
		apply(loader)
	}

	// Set defaults
	if loader.cache == nil {
		loader.cache = NewInMemoryCache[K, V]()
	}

	return loader
}

// Thunk is a function that will block until the value (*Result) it contains is resolved.
// After the value it contains is resolved, this function will return the result.
// This function can be called many times, much like a Promise is other languages.
// The value will only need to be resolved once so subsequent calls will return immediately.
type Thunk[V any] func() (V, error)

// ThunkMany is much like the Thunk func type, but it contains a list of results.
type ThunkMany[V any] func() ([]V, []error)

// Loader batches and caches requests
type Loader[K comparable, V any] struct {
	// this method provides the data for the loader
	fetch BatchFunc[K, V]

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// INTERNAL

	// lazily created cache
	cache Cache[K, V]

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *loaderBatch[K, V]

	// mutex to prevent races
	mu sync.Mutex
}

type loaderBatch[K comparable, V any] struct {
	keys    []K
	data    []V
	error   []error
	closing bool
	done    chan struct{}
}

// Load a Value by key, batching and caching will be applied automatically
func (l *Loader[K, V]) Load(ctx context.Context, key K) (V, error) {
	return l.LoadThunk(ctx, key)()
}

// LoadThunk returns a function that when called will block waiting for a Value.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *Loader[K, V]) LoadThunk(ctx context.Context, key K) Thunk[V] {
	l.mu.Lock()
	if it, ok := l.cache.Get(ctx, key); ok {
		l.mu.Unlock()
		return func() (V, error) {
			return *it, nil
		}
	}
	if l.batch == nil {
		l.batch = &loaderBatch[K, V]{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(ctx, l, key)
	l.mu.Unlock()

	return func() (V, error) {
		<-batch.done

		var data V
		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		// It's convenient to be able to return a single error for everything
		if len(batch.error) == 1 {
			err = batch.error[0]
		} else if batch.error != nil {
			err = batch.error[pos]
		}

		if err == nil {
			l.mu.Lock()
			l.unsafeSet(ctx, key, data)
			l.mu.Unlock()
		}

		return data, err
	}
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *Loader[K, V]) LoadAll(ctx context.Context, keys []K) ([]V, []error) {
	results := make([]func() (V, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(ctx, key)
	}

	values := make([]V, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		values[i], errors[i] = thunk()
	}
	return values, errors
}

// LoadAllThunk returns a function that when called will block waiting for a Value.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *Loader[K, V]) LoadAllThunk(ctx context.Context, keys []K) ThunkMany[V] {
	results := make([]func() (V, error), len(keys))
	for i, key := range keys {
		results[i] = l.LoadThunk(ctx, key)
	}
	return func() ([]V, []error) {
		values := make([]V, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			values[i], errors[i] = thunk()
		}
		return values, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *Loader[K, V]) Prime(ctx context.Context, key K, value V) bool {
	l.mu.Lock()
	var found bool
	if _, found = l.cache.Get(ctx, key); !found {
		l.unsafeSet(ctx, key, value)
	}
	l.mu.Unlock()
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *Loader[K, V]) Clear(ctx context.Context, key K) {
	l.mu.Lock()
	l.cache.Delete(ctx, key)
	l.mu.Unlock()
}

// ClearAll clears the entire cache
func (l *Loader[K, V]) ClearAll(ctx context.Context) {
	l.mu.Lock()
	l.cache.Clear(ctx)
	l.mu.Unlock()
}

func (l *Loader[K, V]) unsafeSet(ctx context.Context, key K, value V) {
	l.cache.Set(ctx, key, value)
}

// keyIndex will return the location of the key in the batch, if it's not found
// it will add the key to the batch
func (b *loaderBatch[K, V]) keyIndex(ctx context.Context, l *Loader[K, V], key K) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go b.startTimer(ctx, l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go b.end(ctx, l)
		}
	}

	return pos
}

func (b *loaderBatch[K, V]) startTimer(ctx context.Context, l *Loader[K, V]) {
	time.Sleep(l.wait)
	l.mu.Lock()

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		l.mu.Unlock()
		return
	}

	l.batch = nil
	l.mu.Unlock()

	b.end(ctx, l)
}

func (b *loaderBatch[K, V]) end(ctx context.Context, l *Loader[K, V]) {
	b.data, b.error = l.fetch(ctx, b.keys)
	close(b.done)
}
