package mcp

type Toolset interface {
	ID() string
	Version() string
	Init(ctx ToolsetContext) error
	Register(reg Registry) error
}
