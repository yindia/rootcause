package mcp

import (
	"errors"
	"sort"
	"sync"
)

type ToolsetFactory func() Toolset

type toolsetRegistry struct {
	mu        sync.RWMutex
	factories map[string]ToolsetFactory
}

var registry = toolsetRegistry{factories: map[string]ToolsetFactory{}}

func RegisterToolset(id string, factory ToolsetFactory) error {
	if id == "" {
		return errors.New("toolset id required")
	}
	if factory == nil {
		return errors.New("toolset factory required")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.factories[id]; exists {
		return errors.New("toolset already registered")
	}
	registry.factories[id] = factory
	return nil
}

func MustRegisterToolset(id string, factory ToolsetFactory) {
	if err := RegisterToolset(id, factory); err != nil {
		panic(err)
	}
}

func ToolsetFactoryFor(id string) (ToolsetFactory, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	factory, ok := registry.factories[id]
	return factory, ok
}

func RegisteredToolsets() []string {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	ids := make([]string, 0, len(registry.factories))
	for id := range registry.factories {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
