# Go TUI AI Agent

High-performance terminal UI agent built with Bubble Tea and Lipgloss.

## Features

- ⚡ Lightning-fast Go implementation
- 🎨 Beautiful terminal UI with Bubble Tea
- 💅 Styled with Lipgloss
- 💾 Low memory footprint
- 🔧 Native HTTP clients (no CGO)
- ⌨️ Keyboard-driven interface
- 📚 Full Obsidian vault integration

## Installation

```bash
# Download dependencies
go mod download

# Build
go build -o obsidian-agent

# Or run directly
go run .
```

## Usage

### Basic Usage

```bash
# With environment variables
export OPENAI_API_KEY="your-key"
export OBSIDIAN_VAULT_PATH="/path/to/vault"
go run .

# Or with compiled binary
./obsidian-agent
```

### Environment Variables

```bash
# Required (at least one)
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"

# Optional
export OBSIDIAN_VAULT_PATH="/path/to/vault"  # Default: ~/Documents/Obsidian
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `Ctrl+P` | Switch provider (OpenAI → Anthropic → Ollama) |
| `Ctrl+N` | Connect to selected provider |
| `Ctrl+C` / `Esc` | Quit application |
| `Enter` | Send message |
| `Backspace` | Delete character |

## Architecture

```
main.go
├── Model (Application State)
│   ├── messages []Message
│   ├── provider Provider
│   ├── tools *ToolRegistry
│   └── vault *ObsidianVault
├── Update (Event Handling)
│   ├── Keyboard Events
│   ├── Message Sending
│   └── Tool Execution
└── View (Rendering)
    ├── Header
    ├── Message History
    └── Input Field

providers.go
├── Provider Interface
├── OpenAIProvider
├── AnthropicProvider
└── OllamaProvider

tools.go
├── Tool struct
└── ToolRegistry

obsidian.go
├── ObsidianVault
└── Obsidian Tools (7 tools)
```

## Building

### Development Build

```bash
go build -o obsidian-agent
```

### Production Build

```bash
# Optimized build
go build -ldflags="-s -w" -o obsidian-agent

# Cross-compilation examples
GOOS=linux GOARCH=amd64 go build -o obsidian-agent-linux
GOOS=darwin GOARCH=arm64 go build -o obsidian-agent-macos
GOOS=windows GOARCH=amd64 go build -o obsidian-agent.exe
```

## Dependencies

```go
require (
    github.com/charmbracelet/bubbletea v0.25.0  // TUI framework
    github.com/charmbracelet/lipgloss v0.9.1    // Styling
)
```

All other dependencies are standard library!

## Adding Custom Tools

```go
// Register in RegisterObsidianTools or create new function
registry.Register(Tool{
    Name: "my_custom_tool",
    Description: "Does something useful",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param1": map[string]interface{}{
                "type": "string",
                "description": "First parameter",
            },
        },
        "required": []string{"param1"},
    },
    Function: func(args map[string]interface{}) (interface{}, error) {
        param1 := args["param1"].(string)
        // Your logic here
        return fmt.Sprintf("Result: %s", param1), nil
    },
})
```

## Provider Configuration

### OpenAI
```go
OpenAIProvider{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4-turbo-preview",
}
```

### Anthropic
```go
AnthropicProvider{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-3-5-sonnet-20241022",
}
```

### Ollama
```go
OllamaProvider{
    BaseURL: "http://localhost:11434",
    Model:   "llama3.1",
}
```

## Performance

The Go implementation is designed for performance:

- **Memory Usage:** ~10-20 MB typical
- **Startup Time:** < 100ms
- **Response Time:** Network-bound (LLM API calls)
- **Concurrent:** Supports async operations

## Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Benchmark
go test -bench=. ./...
```

## Debugging

```bash
# Run with race detector
go run -race .

# Build with debug symbols
go build -gcflags="all=-N -l" -o obsidian-agent

# Profile CPU usage
go run . -cpuprofile=cpu.prof
```

## Styling Customization

Edit styles in `main.go`:

```go
var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#7D56F4"))

    userMessageStyle = lipgloss.NewStyle().
        Background(lipgloss.Color("#7D56F4")).
        Foreground(lipgloss.Color("#FFFFFF"))

    // ... customize other styles
)
```

## Error Handling

The application handles errors gracefully:

- API errors are displayed in the chat
- Network failures show clear messages
- Invalid vault paths warn on startup
- Tool execution errors are caught and displayed

## Troubleshooting

**Issue:** "Not connected to provider"
- **Solution:** Press `Ctrl+N` to connect after selecting provider

**Issue:** "OPENAI_API_KEY not set"
- **Solution:** Export API key: `export OPENAI_API_KEY="your-key"`

**Issue:** Vault not loading
- **Solution:** Check path exists and is readable

**Issue:** UI rendering issues
- **Solution:** Ensure terminal supports ANSI colors and Unicode

## Advanced Usage

### Custom Provider Implementation

```go
type CustomProvider struct {
    APIKey string
    Model  string
}

func (p *CustomProvider) Chat(ctx context.Context, messages []ChatMessage, tools []Tool) (*ChatResponse, error) {
    // Implement your provider logic
    return &ChatResponse{
        Content: "Response",
        ToolCalls: []ToolCall{},
    }, nil
}
```

### Custom Vault Implementation

```go
type CustomVault struct {
    Path string
}

// Implement required methods
func (v *CustomVault) SearchNotes(query string, caseSensitive bool) ([]NoteInfo, error) {
    // Your implementation
}
```

## Deployment

### Binary Distribution

```bash
# Build for all platforms
./build.sh

# Or manually
GOOS=linux GOARCH=amd64 go build -o dist/obsidian-agent-linux
GOOS=darwin GOARCH=arm64 go build -o dist/obsidian-agent-macos
GOOS=windows GOARCH=amd64 go build -o dist/obsidian-agent.exe
```

### System Integration

```bash
# Install to system
sudo cp obsidian-agent /usr/local/bin/

# Create systemd service (optional)
sudo nano /etc/systemd/system/obsidian-agent.service
```

## License

MIT License
