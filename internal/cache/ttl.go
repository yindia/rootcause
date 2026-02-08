package cache

import (
	"sync"
	"time"
)

type entry struct {
	value     any
	expiresAt time.Time
}

type Store struct {
	mu    sync.RWMutex
	items map[string]entry
}

func NewStore() *Store {
	return &Store{items: map[string]entry{}}
}

func (s *Store) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	item, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		s.mu.Lock()
		delete(s.items, key)
		s.mu.Unlock()
		return nil, false
	}
	return item.value, true
}

func (s *Store) Set(key string, value any, ttl time.Duration) {
	if s == nil || key == "" {
		return
	}
	expiry := time.Time{}
	if ttl > 0 {
		expiry = time.Now().Add(ttl)
	}
	s.mu.Lock()
	s.items[key] = entry{value: value, expiresAt: expiry}
	s.mu.Unlock()
}

func (s *Store) Delete(key string) {
	if s == nil || key == "" {
		return
	}
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}
