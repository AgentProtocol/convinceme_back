package tools

import "context"

// Tool is the interface that all tools must implement
type Tool interface {
	// Name returns the name of the tool
	Name() string

	// Description returns a description of what the tool does
	Description() string

	// Call executes the tool with the given input and returns the result
	Call(ctx context.Context, input string) (string, error)
}
