#!/usr/bin/env python3
"""
AI Agent Web Server - FastAPI-based web interface for AI agents with tool calling
Supports multiple LLM providers and custom tools
"""

import os
import json
import sys
from pathlib import Path
from typing import Any, Dict, List, Optional
from dataclasses import dataclass, asdict
from datetime import datetime

from fastapi import FastAPI, HTTPException, WebSocket, WebSocketDisconnect
from fastapi.staticfiles import StaticFiles
from fastapi.responses import HTMLResponse, JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
import uvicorn

# Add shared directory to path
sys.path.insert(0, str(Path(__file__).parent.parent / "shared"))

from obsidian_tools import ObsidianVault, execute_obsidian_tool, get_obsidian_tool_definitions


# Pydantic models for API
class ChatMessage(BaseModel):
    role: str
    content: str


class ChatRequest(BaseModel):
    messages: List[ChatMessage]
    provider: str = "openai"
    model: Optional[str] = None
    use_tools: bool = True


class ChatResponse(BaseModel):
    message: ChatMessage
    tool_calls: List[Dict[str, Any]] = []
    model_used: str


class ProviderConfig(BaseModel):
    provider: str
    model: Optional[str] = None
    api_key: Optional[str] = None


# LLM Provider implementations (reusing from TUI)
class LLMProvider:
    """Base class for LLM providers"""

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        raise NotImplementedError


class OpenAIProvider(LLMProvider):
    """OpenAI API provider"""

    def __init__(self, api_key: Optional[str] = None, model: str = "gpt-4-turbo-preview"):
        self.api_key = api_key or os.getenv("OPENAI_API_KEY")
        self.model = model
        try:
            from openai import OpenAI
            self.client = OpenAI(api_key=self.api_key)
        except ImportError:
            raise ImportError("openai package not installed")

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        kwargs = {"model": self.model, "messages": messages}
        if tools:
            kwargs["tools"] = tools

        response = self.client.chat.completions.create(**kwargs)
        message = response.choices[0].message

        result = {
            "content": message.content or "",
            "tool_calls": [],
            "model": self.model
        }

        if hasattr(message, 'tool_calls') and message.tool_calls:
            for tool_call in message.tool_calls:
                result["tool_calls"].append({
                    "id": tool_call.id,
                    "name": tool_call.function.name,
                    "arguments": json.loads(tool_call.function.arguments)
                })

        return result


class AnthropicProvider(LLMProvider):
    """Anthropic Claude API provider"""

    def __init__(self, api_key: Optional[str] = None, model: str = "claude-3-5-sonnet-20241022"):
        self.api_key = api_key or os.getenv("ANTHROPIC_API_KEY")
        self.model = model
        try:
            from anthropic import Anthropic
            self.client = Anthropic(api_key=self.api_key)
        except ImportError:
            raise ImportError("anthropic package not installed")

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        system_messages = [m["content"] for m in messages if m["role"] == "system"]
        chat_messages = [m for m in messages if m["role"] != "system"]

        kwargs = {
            "model": self.model,
            "max_tokens": 4096,
            "messages": chat_messages
        }

        if system_messages:
            kwargs["system"] = "\n".join(system_messages)

        if tools:
            anthropic_tools = []
            for tool in tools:
                func = tool["function"]
                anthropic_tools.append({
                    "name": func["name"],
                    "description": func["description"],
                    "input_schema": func["parameters"]
                })
            kwargs["tools"] = anthropic_tools

        response = self.client.messages.create(**kwargs)

        result = {
            "content": "",
            "tool_calls": [],
            "model": self.model
        }

        for block in response.content:
            if block.type == "text":
                result["content"] += block.text
            elif block.type == "tool_use":
                result["tool_calls"].append({
                    "id": block.id,
                    "name": block.name,
                    "arguments": block.input
                })

        return result


class OllamaProvider(LLMProvider):
    """Ollama local model provider"""

    def __init__(self, model: str = "llama3.1", base_url: str = "http://localhost:11434"):
        self.model = model
        self.base_url = base_url
        try:
            import requests
            self.requests = requests
        except ImportError:
            raise ImportError("requests package not installed")

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        payload = {
            "model": self.model,
            "messages": messages,
            "stream": False
        }

        if tools:
            payload["tools"] = tools

        response = self.requests.post(f"{self.base_url}/api/chat", json=payload)
        response.raise_for_status()

        data = response.json()
        message = data.get("message", {})

        result = {
            "content": message.get("content", ""),
            "tool_calls": [],
            "model": self.model
        }

        if "tool_calls" in message:
            for tool_call in message["tool_calls"]:
                result["tool_calls"].append({
                    "id": tool_call.get("id", ""),
                    "name": tool_call["function"]["name"],
                    "arguments": tool_call["function"]["arguments"]
                })

        return result


# FastAPI app
app = FastAPI(title="AI Agent Web Server", version="1.0.0")

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Global state
vault: Optional[ObsidianVault] = None
tool_definitions: List[Dict] = []


@app.on_event("startup")
async def startup_event():
    """Initialize vault on startup"""
    global vault, tool_definitions

    vault_path = os.getenv("OBSIDIAN_VAULT_PATH", "~/Documents/Obsidian")
    try:
        vault = ObsidianVault(vault_path)
        tool_definitions, _ = get_obsidian_tool_definitions(str(vault.vault_path))
        print(f"✓ Obsidian vault loaded: {vault.vault_path}")
        print(f"✓ {len(tool_definitions)} tools registered")
    except Exception as e:
        print(f"⚠ Warning: Could not load Obsidian vault: {e}")
        print("  Obsidian tools will not be available")
        vault = None
        tool_definitions = []


@app.get("/", response_class=HTMLResponse)
async def root():
    """Serve the main HTML page"""
    return HTMLResponse(content=get_html_content())


@app.get("/api/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "vault_loaded": vault is not None,
        "tools_available": len(tool_definitions),
        "timestamp": datetime.utcnow().isoformat()
    }


@app.get("/api/tools")
async def get_tools():
    """Get available tools"""
    return {"tools": tool_definitions}


@app.post("/api/chat", response_model=ChatResponse)
async def chat(request: ChatRequest):
    """Chat endpoint with tool calling support"""
    try:
        # Initialize provider
        provider = create_provider(request.provider, request.model)

        # Convert messages
        messages = [{"role": msg.role, "content": msg.content} for msg in request.messages]

        # Get tools if enabled
        tools = tool_definitions if request.use_tools and vault else None

        # Get response
        response = provider.chat(messages, tools=tools)

        # Execute tools if any
        if response["tool_calls"] and vault:
            tool_results = []
            for tool_call in response["tool_calls"]:
                try:
                    result = execute_obsidian_tool(
                        vault,
                        tool_call["name"],
                        tool_call["arguments"]
                    )
                    tool_results.append({
                        "tool_call_id": tool_call["id"],
                        "result": result
                    })
                except Exception as e:
                    tool_results.append({
                        "tool_call_id": tool_call["id"],
                        "error": str(e)
                    })

            # Add tool results to messages and get final response
            messages.append({
                "role": "assistant",
                "content": response.get("content") or "",
                "tool_calls": response["tool_calls"]
            })

            for tool_result in tool_results:
                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_result["tool_call_id"],
                    "content": json.dumps(tool_result.get("result", tool_result.get("error")))
                })

            # Get final response
            final_response = provider.chat(messages)
            response = final_response

        return ChatResponse(
            message=ChatMessage(role="assistant", content=response["content"]),
            tool_calls=response.get("tool_calls", []),
            model_used=response.get("model", "unknown")
        )

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


def create_provider(provider_type: str, model: Optional[str] = None) -> LLMProvider:
    """Create an LLM provider instance"""
    if provider_type == "openai":
        return OpenAIProvider(model=model or "gpt-4-turbo-preview")
    elif provider_type == "anthropic":
        return AnthropicProvider(model=model or "claude-3-5-sonnet-20241022")
    elif provider_type == "ollama":
        return OllamaProvider(model=model or "llama3.1")
    else:
        raise ValueError(f"Unknown provider: {provider_type}")


def get_html_content() -> str:
    """Get the HTML content for the web interface"""
    return """
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>AI Agent - Obsidian Assistant</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            justify-content: center;
            align-items: center;
            padding: 20px;
        }

        .container {
            background: white;
            border-radius: 20px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            width: 100%;
            max-width: 1200px;
            height: 80vh;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }

        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .header h1 {
            font-size: 24px;
            font-weight: 600;
        }

        .provider-select {
            padding: 8px 16px;
            border-radius: 8px;
            border: 2px solid white;
            background: rgba(255, 255, 255, 0.2);
            color: white;
            font-size: 14px;
            cursor: pointer;
        }

        .chat-container {
            flex: 1;
            overflow-y: auto;
            padding: 20px;
            background: #f8f9fa;
        }

        .message {
            margin-bottom: 16px;
            display: flex;
            gap: 12px;
        }

        .message.user {
            flex-direction: row-reverse;
        }

        .message-content {
            max-width: 70%;
            padding: 12px 16px;
            border-radius: 12px;
            line-height: 1.5;
        }

        .message.user .message-content {
            background: #667eea;
            color: white;
        }

        .message.assistant .message-content {
            background: white;
            border: 1px solid #e0e0e0;
            color: #333;
        }

        .message.system .message-content {
            background: #fff3cd;
            border: 1px solid #ffc107;
            color: #856404;
            max-width: 100%;
        }

        .tool-call {
            background: #e7f3ff;
            border: 1px solid #0066cc;
            border-radius: 8px;
            padding: 8px 12px;
            margin: 8px 0;
            font-family: 'Courier New', monospace;
            font-size: 12px;
        }

        .input-container {
            padding: 20px;
            background: white;
            border-top: 1px solid #e0e0e0;
            display: flex;
            gap: 12px;
        }

        #messageInput {
            flex: 1;
            padding: 12px 16px;
            border: 2px solid #e0e0e0;
            border-radius: 12px;
            font-size: 14px;
            outline: none;
            transition: border-color 0.3s;
        }

        #messageInput:focus {
            border-color: #667eea;
        }

        #sendBtn {
            padding: 12px 32px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 12px;
            font-size: 14px;
            font-weight: 600;
            cursor: pointer;
            transition: transform 0.2s;
        }

        #sendBtn:hover {
            transform: translateY(-2px);
        }

        #sendBtn:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }

        .avatar {
            width: 36px;
            height: 36px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: bold;
            flex-shrink: 0;
        }

        .message.user .avatar {
            background: #667eea;
            color: white;
        }

        .message.assistant .avatar {
            background: #764ba2;
            color: white;
        }

        .loading {
            display: none;
            text-align: center;
            padding: 20px;
            color: #666;
        }

        .loading.show {
            display: block;
        }

        .status-indicator {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            display: inline-block;
            margin-right: 8px;
        }

        .status-indicator.connected {
            background: #4caf50;
        }

        .status-indicator.disconnected {
            background: #f44336;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>
                <span class="status-indicator" id="statusIndicator"></span>
                AI Agent - Obsidian Assistant
            </h1>
            <select class="provider-select" id="providerSelect">
                <option value="openai">OpenAI (GPT-4)</option>
                <option value="anthropic">Anthropic (Claude)</option>
                <option value="ollama">Ollama (Local)</option>
            </select>
        </div>

        <div class="chat-container" id="chatContainer">
            <div class="message system">
                <div class="message-content">
                    Welcome to AI Agent! I'm connected to your Obsidian vault and ready to help.
                    I can search notes, create new notes, find backlinks, and more.
                </div>
            </div>
        </div>

        <div class="loading" id="loading">
            <span>Thinking...</span>
        </div>

        <div class="input-container">
            <input type="text" id="messageInput" placeholder="Ask me anything about your Obsidian vault..." />
            <button id="sendBtn">Send</button>
        </div>
    </div>

    <script>
        const chatContainer = document.getElementById('chatContainer');
        const messageInput = document.getElementById('messageInput');
        const sendBtn = document.getElementById('sendBtn');
        const providerSelect = document.getElementById('providerSelect');
        const loading = document.getElementById('loading');
        const statusIndicator = document.getElementById('statusIndicator');

        let messages = [];

        // Check health on load
        async function checkHealth() {
            try {
                const response = await fetch('/api/health');
                const data = await response.json();
                statusIndicator.className = 'status-indicator ' + (data.vault_loaded ? 'connected' : 'disconnected');
            } catch (error) {
                statusIndicator.className = 'status-indicator disconnected';
            }
        }

        checkHealth();
        setInterval(checkHealth, 30000);

        function addMessage(role, content, toolCalls = []) {
            const messageDiv = document.createElement('div');
            messageDiv.className = `message ${role}`;

            const avatar = document.createElement('div');
            avatar.className = 'avatar';
            avatar.textContent = role === 'user' ? 'You' : 'AI';

            const contentDiv = document.createElement('div');
            contentDiv.className = 'message-content';

            if (toolCalls && toolCalls.length > 0) {
                const toolInfo = document.createElement('div');
                toolInfo.className = 'tool-call';
                toolInfo.textContent = `🔧 Used tools: ${toolCalls.map(t => t.name).join(', ')}`;
                contentDiv.appendChild(toolInfo);
            }

            const text = document.createElement('div');
            text.textContent = content;
            contentDiv.appendChild(text);

            messageDiv.appendChild(avatar);
            messageDiv.appendChild(contentDiv);

            chatContainer.appendChild(messageDiv);
            chatContainer.scrollTop = chatContainer.scrollHeight;
        }

        async function sendMessage() {
            const message = messageInput.value.trim();
            if (!message) return;

            messageInput.value = '';
            sendBtn.disabled = true;
            loading.classList.add('show');

            addMessage('user', message);
            messages.push({ role: 'user', content: message });

            try {
                const response = await fetch('/api/chat', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        messages: messages,
                        provider: providerSelect.value,
                        use_tools: true
                    })
                });

                if (!response.ok) {
                    throw new Error('Failed to get response');
                }

                const data = await response.json();
                addMessage('assistant', data.message.content, data.tool_calls);
                messages.push({ role: 'assistant', content: data.message.content });

            } catch (error) {
                addMessage('system', `Error: ${error.message}`);
            } finally {
                sendBtn.disabled = false;
                loading.classList.remove('show');
            }
        }

        sendBtn.addEventListener('click', sendMessage);
        messageInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                sendMessage();
            }
        });
    </script>
</body>
</html>
"""


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="AI Agent Web Server")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    parser.add_argument("--port", type=int, default=8000, help="Port to bind to")

    args = parser.parse_args()

    print(f"🚀 Starting AI Agent Web Server on http://{args.host}:{args.port}")
    print(f"📝 Set OBSIDIAN_VAULT_PATH environment variable to your vault location")

    uvicorn.run(app, host=args.host, port=args.port)
