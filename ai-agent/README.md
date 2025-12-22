# AI Agent System - Multiple Implementation Approaches

A comprehensive exploration of different ways to build AI agent systems with tool calling support, specialized for Obsidian vault integration.

## 🎯 Project Overview

This repository contains **four different implementations** of AI agent systems:

1. **Python TUI Agent** - Terminal UI using Textual
2. **Python Web Agent** - Web-based using FastAPI
3. **Go TUI Agent** - High-performance terminal UI using Bubble Tea
4. **MCP Server** - Model Context Protocol compliant server

All implementations support:
- ✅ Multiple LLM providers (OpenAI, Anthropic Claude, Ollama)
- ✅ Tool calling with custom tool framework
- ✅ Obsidian vault integration (7 specialized tools)
- ✅ Extensible architecture
- ✅ Production-ready code

## 📋 Quick Links

- **[Full HTML Documentation](index.html)** - Open in browser for comprehensive guide
- **[Research Findings](#research-findings)** - MCP and LSP insights
- **[Getting Started](#getting-started)** - Quick setup guide
- **[Implementation Details](#implementations)** - Detailed breakdown

## 🔬 Research Findings

### Model Context Protocol (MCP)

**Key Findings:**
- Open standard by Anthropic (Nov 2024) for standardizing AI tool integration
- Industry-wide adoption: OpenAI (March 2025), Google DeepMind (April 2025)
- "USB-C for AI applications" - universal connection standard
- Official SDKs: Python, TypeScript, C#, Ruby
- Community SDKs: Go (go-go-mcp, gomcp)

**Benefits:**
- Standardized tool calling across providers
- Efficient resource loading and context management
- Growing ecosystem with thousands of servers

### Language Server Protocol (LSP) Integration

**Key Findings:**
- Agent Client Protocol (ACP) emerging as "LSP for AI agents"
- LSP-MCP bridges allow code intelligence in AI assistants
- LSP-AI provides open-source AI-powered code assistance
- Integration enables diagnostics, completion, and code actions

## 🚀 Implementations

### 1. Python TUI Agent (`python-tui/`)

**Best for:** CLI power users, quick prototyping, personal use

**Features:**
- Beautiful terminal UI with Textual framework
- Real-time chat with markdown rendering
- Provider switching at runtime
- Visual tool execution feedback

**Quick Start:**
```bash
cd python-tui
pip install -r requirements.txt
export OPENAI_API_KEY="your-key"
export OBSIDIAN_VAULT_PATH="/path/to/vault"
python obsidian_agent.py
```

### 2. Python Web Agent (`python-web/`)

**Best for:** Team collaboration, web access, multi-user scenarios

**Features:**
- Modern web interface with gradient design
- RESTful API with FastAPI
- Health monitoring endpoints
- Real-time chat updates
- Multi-user capable

**Quick Start:**
```bash
cd python-web
pip install -r requirements.txt
export OPENAI_API_KEY="your-key"
export OBSIDIAN_VAULT_PATH="/path/to/vault"
python server.py --host 0.0.0.0 --port 8000
# Open http://localhost:8000
```

### 3. Go TUI Agent (`go-tui/`)

**Best for:** Performance-critical applications, minimal dependencies

**Features:**
- Lightning-fast Go implementation
- Bubble Tea framework for TUI
- Lipgloss styling
- Low memory footprint
- Native HTTP clients (no external dependencies)

**Quick Start:**
```bash
cd go-tui
go mod download
export OPENAI_API_KEY="your-key"
export OBSIDIAN_VAULT_PATH="/path/to/vault"
go run .

# Keyboard shortcuts:
# Ctrl+P - Switch provider
# Ctrl+N - Connect
# Ctrl+C - Quit
```

### 4. MCP Server (`mcp-server/`)

**Best for:** Standard integrations, Claude Desktop, protocol compliance

**Features:**
- MCP protocol compliant
- Stdio-based communication
- Works with any MCP client
- Exposes tools and resources
- Claude Desktop integration

**Quick Start:**
```bash
cd mcp-server
pip install -r requirements.txt
python obsidian_mcp_server.py /path/to/vault

# Or test with example client:
python mcp_client_example.py
```

**Claude Desktop Integration:**
Add to `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "obsidian": {
      "command": "python",
      "args": ["/absolute/path/to/obsidian_mcp_server.py"],
      "env": {
        "OBSIDIAN_VAULT_PATH": "/path/to/vault"
      }
    }
  }
}
```

## 📚 Obsidian Tools

All implementations include these tools:

| Tool | Description |
|------|-------------|
| `search_obsidian_notes` | Search notes with optional case sensitivity |
| `read_obsidian_note` | Read complete note contents with frontmatter |
| `create_obsidian_note` | Create new note with tags and folder |
| `update_obsidian_note` | Update note (replace or append mode) |
| `list_obsidian_notes` | List notes sorted by modification date |
| `get_obsidian_backlinks` | Find notes linking to a specific note |
| `get_obsidian_tags` | Get all tags with frequency counts |

## ⚖️ Comparison Matrix

| Feature | Python TUI | Python Web | Go TUI | MCP Server |
|---------|-----------|-----------|--------|-----------|
| **Interface** | Terminal | Web Browser | Terminal | Protocol |
| **Performance** | Fast | Good | Very Fast | Fast |
| **Multi-user** | No | Yes | No | No |
| **Deployment** | Simple | Server | Simple | Simple |
| **Memory** | Medium | Medium-High | Low | Low |
| **Dependencies** | Moderate | Moderate | Minimal | Light |

## 🏗️ Architecture

```
┌─────────────────────────────────────────────┐
│         User Interface Layer                │
│  TUI / Web / Protocol                       │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│         Agent Core Layer                    │
│  Messages / Provider Abstraction / Tools    │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│         Provider Layer                      │
│  OpenAI / Anthropic / Ollama               │
└─────────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────────┐
│         Tool Layer                          │
│  Obsidian Tools / Custom Tools             │
└─────────────────────────────────────────────┘
```

## 🛠️ Getting Started

### Prerequisites

- Python 3.9+ (for Python implementations)
- Go 1.21+ (for Go implementation)
- API keys for chosen providers
- Obsidian vault (optional)

### Environment Setup

```bash
# Required for cloud providers
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"

# Optional: vault location (defaults to ~/Documents/Obsidian)
export OBSIDIAN_VAULT_PATH="/path/to/vault"

# Optional: Ollama URL (defaults to localhost:11434)
export OLLAMA_BASE_URL="http://localhost:11434"
```

### Adding Custom Tools

**Python:**
```python
from agent import Tool

tool = Tool(
    name="my_tool",
    description="Does something",
    parameters={
        "type": "object",
        "properties": {
            "param": {"type": "string"}
        }
    },
    function=lambda param: f"Result: {param}"
)

tool_registry.register(tool)
```

**Go:**
```go
registry.Register(Tool{
    Name: "my_tool",
    Description: "Does something",
    Parameters: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "param": map[string]interface{}{
                "type": "string",
            },
        },
    },
    Function: func(args map[string]interface{}) (interface{}, error) {
        return fmt.Sprintf("Result: %s", args["param"]), nil
    },
})
```

## 📂 Repository Structure

```
ai-agent/
├── python-tui/              # Python Terminal UI
│   ├── agent.py            # Base agent
│   ├── obsidian_agent.py   # Obsidian-specialized
│   └── requirements.txt
├── python-web/              # Python Web
│   ├── server.py           # FastAPI server
│   └── requirements.txt
├── go-tui/                  # Go Terminal UI
│   ├── main.go
│   ├── providers.go
│   ├── tools.go
│   ├── obsidian.go
│   └── go.mod
├── mcp-server/              # MCP Server
│   ├── obsidian_mcp_server.py
│   ├── mcp_client_example.py
│   ├── mcp_config.json
│   └── requirements.txt
├── shared/                  # Shared utilities
│   └── obsidian_tools.py
├── index.html              # Full documentation
└── README.md               # This file
```

## 🔮 Future Enhancements

- [ ] Streaming responses
- [ ] Conversation persistence
- [ ] User authentication (web)
- [ ] Usage analytics
- [ ] Additional integrations (GitHub, Jira, Slack)
- [ ] Comprehensive test suite
- [ ] Docker containers
- [ ] Kubernetes deployment configs

## 📖 Documentation

For the full interactive documentation with examples, architecture diagrams, and detailed comparisons, open `index.html` in your browser.

## 🤝 Contributing

Each implementation is independent and can be extended separately:
- Add new providers by implementing the Provider interface
- Add new tools by registering them with the Tool Registry
- Create new UIs using the core agent logic
- Extend Obsidian tools with additional functionality

## 📝 License

MIT License - feel free to use and modify for your needs.

## 🙏 Acknowledgments

- **Anthropic** for the Model Context Protocol
- **OpenAI** for GPT models and API
- **Textual** for Python TUI framework
- **Bubble Tea** for Go TUI framework
- **FastAPI** for Python web framework

---

**Choose the implementation that fits your needs, or use them all!** Each one demonstrates different approaches to building AI agents with tool calling support.
