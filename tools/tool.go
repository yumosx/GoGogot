package tools

import "context"

type Result struct {
	Output string
	IsErr  bool
}

type Handler func(ctx context.Context, input map[string]any) Result

type Tool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Required    []string
	Handler     Handler
}
