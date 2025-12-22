# Python Web AI Agent

Web-based AI agent with FastAPI backend and modern HTML/JavaScript frontend.

## Features

- 🌐 Modern web interface with gradient design
- 🔌 RESTful API
- 💬 Real-time chat updates
- 🏥 Health monitoring endpoints
- 👥 Multi-user capable
- 📚 Full Obsidian vault integration

## Installation

```bash
pip install -r requirements.txt
```

## Usage

### Start Server

```bash
# Basic usage
python server.py

# Custom host and port
python server.py --host 0.0.0.0 --port 8080

# With environment variables
export OBSIDIAN_VAULT_PATH="/path/to/vault"
export OPENAI_API_KEY="your-key"
python server.py
```

### Access Web Interface

Open your browser to: `http://localhost:8000`

## API Endpoints

### GET /
Returns the web interface HTML

### GET /api/health
Health check endpoint
```json
{
  "status": "healthy",
  "vault_loaded": true,
  "tools_available": 7,
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### GET /api/tools
Get available tools
```json
{
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "search_obsidian_notes",
        "description": "...",
        "parameters": {...}
      }
    }
  ]
}
```

### POST /api/chat
Send chat message with tool calling

**Request:**
```json
{
  "messages": [
    {"role": "user", "content": "Search for Python notes"}
  ],
  "provider": "openai",
  "model": "gpt-4-turbo-preview",
  "use_tools": true
}
```

**Response:**
```json
{
  "message": {
    "role": "assistant",
    "content": "I found 5 notes about Python..."
  },
  "tool_calls": [
    {
      "id": "call_123",
      "name": "search_obsidian_notes",
      "arguments": {"query": "Python"}
    }
  ],
  "model_used": "gpt-4-turbo-preview"
}
```

## Environment Variables

```bash
# Required (at least one)
export OPENAI_API_KEY="your-openai-key"
export ANTHROPIC_API_KEY="your-anthropic-key"

# Optional
export OBSIDIAN_VAULT_PATH="/path/to/vault"  # Default: ~/Documents/Obsidian
```

## Architecture

```
FastAPI Server
├── Static Frontend (embedded HTML/JS)
├── REST API Endpoints
│   ├── /api/health
│   ├── /api/tools
│   └── /api/chat
├── Provider Layer
│   ├── OpenAIProvider
│   ├── AnthropicProvider
│   └── OllamaProvider
└── Tool Layer
    └── Obsidian Tools
```

## Deployment

### Production Deployment

```bash
# Using Gunicorn
pip install gunicorn
gunicorn -w 4 -k uvicorn.workers.UvicornWorker server:app

# Using Docker
docker build -t ai-agent-web .
docker run -p 8000:8000 \
  -e OPENAI_API_KEY=your-key \
  -e OBSIDIAN_VAULT_PATH=/vault \
  -v /path/to/vault:/vault \
  ai-agent-web
```

### CORS Configuration

The server allows all origins by default. For production, modify:

```python
app.add_middleware(
    CORSMiddleware,
    allow_origins=["https://yourdomain.com"],  # Restrict origins
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)
```

## Dependencies

- `fastapi>=0.109.0` - Web framework
- `uvicorn[standard]>=0.27.0` - ASGI server
- `pydantic>=2.6.0` - Data validation
- `openai>=1.12.0` - OpenAI API
- `anthropic>=0.18.0` - Anthropic API
- `requests>=2.31.0` - HTTP library
- `pyyaml>=6.0.1` - YAML parsing

## Customization

### Custom Styling

Edit the embedded CSS in `server.py` in the `get_html_content()` function.

### Custom Tools

```python
# Add to startup_event
@app.on_event("startup")
async def startup_event():
    global vault, tool_definitions

    # Load vault
    vault = ObsidianVault(vault_path)

    # Add custom tools
    custom_tool = {
        "type": "function",
        "function": {
            "name": "custom_tool",
            "description": "Custom functionality",
            "parameters": {...}
        }
    }
    tool_definitions.append(custom_tool)
```

## Monitoring

The `/api/health` endpoint can be used for monitoring:

```bash
# Check health
curl http://localhost:8000/api/health

# Prometheus-style monitoring (add endpoint)
curl http://localhost:8000/metrics
```

## Security Considerations

⚠️ **Important for Production:**

1. Add authentication middleware
2. Restrict CORS origins
3. Use HTTPS
4. Implement rate limiting
5. Validate and sanitize all inputs
6. Set up proper logging
7. Use environment-specific configs

## Troubleshooting

**Issue:** Port already in use
- **Solution:** Use `--port` flag with different port

**Issue:** Vault not loading
- **Solution:** Check `OBSIDIAN_VAULT_PATH` is correct and accessible

**Issue:** CORS errors
- **Solution:** Check CORS middleware configuration
