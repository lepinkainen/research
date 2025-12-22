package main

import (
	"fmt"
)

// Tool represents a tool that can be called by the AI
type Tool struct {
	Name        string
	Description string
	Parameters  map[string]interface{}
	Function    func(map[string]interface{}) (interface{}, error)
}

// ToolRegistry manages available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name] = tool
}

// GetToolDefinitions returns tool definitions for API calls
func (r *ToolRegistry) GetToolDefinitions() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ExecuteTool executes a tool by name
func (r *ToolRegistry) ExecuteTool(name string, arguments map[string]interface{}) (interface{}, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	return tool.Function(arguments)
}
