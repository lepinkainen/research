# Python TUI AI Agent

Terminal-based AI agent with rich text UI using the Textual framework.

## Features

- 🎨 Beautiful terminal UI with Textual
- 💬 Real-time chat interface with markdown rendering
- 🔄 Runtime provider switching (OpenAI/Anthropic/Ollama)
- 🔧 Visual tool execution feedback
- 📚 Full Obsidian vault integration
- ⌨️ Keyboard-driven interface

## Installation

```bash
pip install -r requirements.txt
```

## Usage

### Basic Usage

```bash
# With default vault location (~/Documents/Obsidian)
python agent.py

# With custom vault location
export OBSIDIAN_VAULT_PATH="/path/to/vault"
python obsidian_agent.py
```

### Environment Variables

```bash
# Required (at least one)
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"

# Optional
export OBSIDIAN_VAULT_PATH="/path/to/vault"  # Default: ~/Documents/Obsidian
export OLLAMA_BASE_URL="http://localhost:11434"  # For Ollama
```

### Keyboard Shortcuts

- `Ctrl+C` - Quit application
- `Ctrl+L` - Clear chat history
- `Enter` - Send message

## Architecture

```
ObsidianAgentTUI
├── Provider Management
│   ├── OpenAIProvider
│   ├── AnthropicProvider
│   └── OllamaProvider
├── Tool Registry
│   └── Obsidian Tools (7 tools)
└── Textual UI
    ├── Chat Log (RichLog)
    ├── Provider Select
    └── Input Field
```

## Adding Custom Tools

```python
from agent import Tool

# Define your tool
custom_tool = Tool(
    name="custom_action",
    description="Performs a custom action",
    parameters={
        "type": "object",
        "properties": {
            "param": {
                "type": "string",
                "description": "Parameter description"
            }
        },
        "required": ["param"]
    },
    function=lambda param: f"Executed: {param}"
)

# In your agent subclass
class CustomAgent(AgentTUI):
    def register_tools(self):
        super().register_tools()  # Register base tools
        self.tool_registry.register(custom_tool)
```

## Provider Configuration

### OpenAI
- Model: `gpt-4-turbo-preview`
- Requires: `OPENAI_API_KEY`

### Anthropic
- Model: `claude-3-5-sonnet-20241022`
- Requires: `ANTHROPIC_API_KEY`

### Ollama (Local)
- Default Model: `llama3.1`
- Default URL: `http://localhost:11434`
- Customizable via environment variables

## Dependencies

- `textual>=0.47.0` - TUI framework
- `rich>=13.7.0` - Terminal formatting
- `openai>=1.12.0` - OpenAI API
- `anthropic>=0.18.0` - Anthropic API
- `requests>=2.31.0` - HTTP library
- `pyyaml>=6.0.1` - YAML parsing

## Troubleshooting

**Issue:** "Provider not connected"
- **Solution:** Click "Connect" button after selecting provider

**Issue:** "Tool not found"
- **Solution:** Ensure vault path is correct and accessible

**Issue:** "API error"
- **Solution:** Check that API keys are set correctly
