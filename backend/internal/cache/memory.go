package cache

import (
	"sync"
	"time"
)

type MemoryCache[K comparable, V any] struct {
	mu       sync.RWMutex
	ttl      time.Duration
	items    map[K]entry[V]
	stopChan chan struct{}
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

func NewMemoryCache[K comparable, V any](ttl time.Duration) *MemoryCache[K, V] {
	return &MemoryCache[K, V]{
		ttl:      ttl,
		items:    make(map[K]entry[V]),
		stopChan: make(chan struct{}),
	}
}

func (c *MemoryCache[K, V]) Get(key K) (V, bool) {
	var zero V
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return zero, false
	}
	return e.value, true
}

func (c *MemoryCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	c.items[key] = entry[V]{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *MemoryCache[K, V]) Delete(key K) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

func (c *MemoryCache[K, V]) StartJanitor() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.items {
				if now.After(e.expiresAt) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopChan:
			return
		}
	}
}

func (c *MemoryCache[K, V]) StopJanitor() { close(c.stopChan) }
