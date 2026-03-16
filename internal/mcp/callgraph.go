package mcp

import "sync"

type CallGraph struct {
	mu    sync.RWMutex
	edges map[string]map[string]int
}

func NewCallGraph() *CallGraph {
	return &CallGraph{edges: map[string]map[string]int{}}
}

func (g *CallGraph) Record(from, to string) {
	if g == nil || from == "" || to == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.edges[from]; !ok {
		g.edges[from] = map[string]int{}
	}
	g.edges[from][to]++
}

func (g *CallGraph) Edges() []map[string]any {
	if g == nil {
		return nil
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]map[string]any, 0)
	for from, targets := range g.edges {
		for to, count := range targets {
			out = append(out, map[string]any{"from": from, "to": to, "count": count, "observed": true})
		}
	}
	return out
}
