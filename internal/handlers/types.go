package handlers

// TextContent is a text-type MCP content item.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema ToolInputSchema `json:"inputSchema"`
}

// ToolInputSchema is the JSON Schema for a tool's input.
type ToolInputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]SchemaProperty `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// SchemaProperty describes a single property in a JSON Schema.
type SchemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}
