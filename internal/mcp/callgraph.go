package mcp

import "sync"

type CallGraph struct {
	mu       sync.RWMutex
	edges    map[string]map[string]int
	maxNodes int
	nodes    int
}

func NewCallGraph(maxEdges int) *CallGraph {
	return &CallGraph{edges: map[string]map[string]int{}, maxNodes: maxEdges}
}

func (g *CallGraph) Record(from, to string) {
	if g == nil || from == "" || to == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if existing, ok := g.edges[from][to]; ok {
		g.edges[from][to] = existing + 1
		return
	}
	if g.maxNodes > 0 && g.nodes >= g.maxNodes {
		return
	}
	if _, ok := g.edges[from]; !ok {
		g.edges[from] = map[string]int{}
	}
	g.edges[from][to] = 1
	g.nodes++
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
