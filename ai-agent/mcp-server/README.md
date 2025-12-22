# Obsidian MCP Server

Model Context Protocol (MCP) server for Obsidian vault integration, compatible with Claude Desktop and other MCP clients.

## Features

- 🔌 MCP protocol compliant
- 📡 Stdio-based communication
- 🔧 7 Obsidian-specific tools
- 📚 Resource endpoints for notes
- 🤖 Works with any MCP client
- 💻 Claude Desktop integration

## What is MCP?

The Model Context Protocol (MCP) is an open standard introduced by Anthropic to standardize how AI assistants connect to external data sources and tools. It's like "USB-C for AI applications."

**Learn more:** https://modelcontextprotocol.io/

## Installation

```bash
pip install -r requirements.txt
```

## Usage

### As Standalone Server

```bash
# Basic usage
python obsidian_mcp_server.py /path/to/vault

# With environment variable
export OBSIDIAN_VAULT_PATH="/path/to/vault"
python obsidian_mcp_server.py
```

### With Example Client

```bash
export ANTHROPIC_API_KEY="your-key"
export OBSIDIAN_VAULT_PATH="/path/to/vault"
python mcp_client_example.py
```

### With Claude Desktop

1. Find your Claude Desktop config file:
   - **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
   - **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

2. Add the server configuration:

```json
{
  "mcpServers": {
    "obsidian": {
      "command": "python",
      "args": [
        "/absolute/path/to/ai-agent/mcp-server/obsidian_mcp_server.py"
      ],
      "env": {
        "OBSIDIAN_VAULT_PATH": "/path/to/your/vault"
      }
    }
  }
}
```

3. Restart Claude Desktop

4. You should see the 🔌 icon indicating the server is connected

## MCP Protocol

### Tools

The server exposes these tools via MCP:

| Tool Name | Description |
|-----------|-------------|
| `search_notes` | Search for notes containing text |
| `read_note` | Read complete note contents |
| `create_note` | Create a new note |
| `update_note` | Update existing note |
| `list_notes` | List all notes |
| `get_backlinks` | Find notes linking to a note |
| `get_tags` | Get all tags with frequencies |

### Resources

The server exposes recent notes as resources:

- **URI Format:** `obsidian:///path/to/note.md`
- **MIME Type:** `text/markdown`
- **Content:** Full note content

### Example Tool Calls

**Search Notes:**
```json
{
  "name": "search_notes",
  "arguments": {
    "query": "Python",
    "case_sensitive": false
  }
}
```

**Create Note:**
```json
{
  "name": "create_note",
  "arguments": {
    "title": "Meeting Notes",
    "content": "# Meeting with team...",
    "folder": "Work",
    "tags": ["meeting", "work"]
  }
}
```

## Architecture

```
ObsidianMCPServer
├── MCP Protocol Handler
│   ├── list_tools() → Tool Definitions
│   ├── call_tool() → Tool Execution
│   ├── list_resources() → Available Notes
│   └── read_resource() → Note Content
├── Obsidian Vault Interface
│   └── ObsidianVault (from shared/)
└── Stdio Transport
    ├── Read Stream
    └── Write Stream
```

## Client Example

The included `mcp_client_example.py` demonstrates how to:

1. Connect to the MCP server
2. List available tools
3. Send chat messages to Claude
4. Execute tools via MCP
5. Handle tool results

### Running the Example

```bash
export ANTHROPIC_API_KEY="your-key"
export OBSIDIAN_VAULT_PATH="/path/to/vault"
python mcp_client_example.py
```

The example runs through some queries and then enters interactive mode.

## Development

### Creating Custom MCP Servers

Use this server as a template:

```python
from mcp.server import Server
from mcp.server.stdio import stdio_server

server = Server("my-mcp-server")

@server.list_tools()
async def list_tools():
    return [
        Tool(
            name="my_tool",
            description="Does something",
            inputSchema={...}
        )
    ]

@server.call_tool()
async def call_tool(name: str, arguments: Any):
    # Execute tool
    result = do_something(arguments)
    return [TextContent(type="text", text=json.dumps(result))]

async def main():
    async with stdio_server() as (read, write):
        await server.run(read, write, server.create_initialization_options())

if __name__ == "__main__":
    asyncio.run(main())
```

### Testing

```bash
# Unit tests
python -m pytest test_mcp_server.py

# Manual testing with client
python mcp_client_example.py
```

### Debugging

Enable debug logging:

```python
import logging
logging.basicConfig(level=logging.DEBUG)
```

## Configuration

### Environment Variables

```bash
# Vault location (required)
export OBSIDIAN_VAULT_PATH="/path/to/vault"

# Optional: Logging level
export LOG_LEVEL="DEBUG"
```

### Config File

For Claude Desktop, edit `mcp_config.json`:

```json
{
  "mcpServers": {
    "obsidian": {
      "command": "python",
      "args": ["/path/to/obsidian_mcp_server.py"],
      "env": {
        "OBSIDIAN_VAULT_PATH": "/path/to/vault",
        "LOG_LEVEL": "INFO"
      }
    }
  }
}
```

## Dependencies

- `mcp>=0.9.0` - Official MCP Python SDK
- `anthropic>=0.18.0` - For client example
- `pyyaml>=6.0.1` - YAML parsing for frontmatter

## Integration with Other Tools

### Cursor IDE

Add to Cursor MCP settings:

```json
{
  "mcp": {
    "servers": {
      "obsidian": {
        "command": "python",
        "args": ["/path/to/obsidian_mcp_server.py"],
        "env": {
          "OBSIDIAN_VAULT_PATH": "/path/to/vault"
        }
      }
    }
  }
}
```

### Custom Clients

```python
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

# Connect to server
server_params = StdioServerParameters(
    command="python",
    args=["obsidian_mcp_server.py"],
    env={"OBSIDIAN_VAULT_PATH": "/path/to/vault"}
)

transport = await stdio_client(server_params)
read, write = transport
session = ClientSession(read, write)

await session.initialize()

# List tools
tools = await session.list_tools()

# Call tool
result = await session.call_tool("search_notes", {"query": "Python"})
```

## Troubleshooting

**Issue:** Server not appearing in Claude Desktop
- **Solution:** Check config file path and JSON syntax
- **Solution:** Restart Claude Desktop completely
- **Solution:** Check server logs

**Issue:** "Vault path does not exist"
- **Solution:** Use absolute paths in configuration
- **Solution:** Verify vault path is accessible

**Issue:** Tools not executing
- **Solution:** Check server logs for errors
- **Solution:** Verify tool arguments match schema

**Issue:** Connection errors
- **Solution:** Ensure Python and dependencies are installed
- **Solution:** Check file permissions on server script

## Advanced Usage

### Multiple Vaults

Configure multiple servers in Claude Desktop:

```json
{
  "mcpServers": {
    "obsidian-personal": {
      "command": "python",
      "args": ["/path/to/obsidian_mcp_server.py"],
      "env": {
        "OBSIDIAN_VAULT_PATH": "/path/to/personal/vault"
      }
    },
    "obsidian-work": {
      "command": "python",
      "args": ["/path/to/obsidian_mcp_server.py"],
      "env": {
        "OBSIDIAN_VAULT_PATH": "/path/to/work/vault"
      }
    }
  }
}
```

### Custom Tool Addition

Extend the server with custom tools:

```python
@server.list_tools()
async def list_tools() -> list[Tool]:
    base_tools = [...]  # Base Obsidian tools

    # Add custom tool
    base_tools.append(
        Tool(
            name="custom_action",
            description="Performs custom action",
            inputSchema={...}
        )
    )

    return base_tools
```

## Performance

- **Startup Time:** < 1 second
- **Tool Execution:** Depends on vault size
- **Memory Usage:** ~20-30 MB
- **Concurrent Requests:** Single client at a time (MCP design)

## Security Considerations

⚠️ **Important:**

1. The server has full read/write access to your vault
2. Use absolute paths in configuration
3. Validate all tool inputs
4. Run with minimal permissions
5. Review tool execution logs
6. Keep dependencies updated

## Resources

- **MCP Specification:** https://spec.modelcontextprotocol.io/
- **MCP Python SDK:** https://github.com/modelcontextprotocol/python-sdk
- **Claude Desktop:** https://claude.ai/download
- **Anthropic Documentation:** https://docs.anthropic.com/

## License

MIT License
