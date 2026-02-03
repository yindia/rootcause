package mcp

import (
	"errors"
	"sort"
	"sync"
)

type ServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]any
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{services: map[string]any{}}
}

func (r *ServiceRegistry) Register(name string, svc any) error {
	if r == nil {
		return errors.New("service registry is nil")
	}
	if name == "" {
		return errors.New("service name required")
	}
	if svc == nil {
		return errors.New("service value required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.services[name]; exists {
		return errors.New("service already registered")
	}
	r.services[name] = svc
	return nil
}

func (r *ServiceRegistry) Get(name string) (any, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[name]
	return svc, ok
}

func (r *ServiceRegistry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.services))
	for key := range r.services {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
