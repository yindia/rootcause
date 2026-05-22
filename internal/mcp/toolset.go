package mcp

type Toolset interface {
	ID() string
	Version() string
	Init(ctx ToolContext) error
	Register(reg Registry) error
}
