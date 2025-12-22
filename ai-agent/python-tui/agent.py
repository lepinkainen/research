#!/usr/bin/env python3
"""
AI Agent TUI - Terminal User Interface for AI Agents with Tool Calling
Supports multiple LLM providers: OpenAI, Anthropic, and local models via Ollama
"""

import json
import os
from typing import Any, Callable, Dict, List, Optional
from dataclasses import dataclass
from abc import ABC, abstractmethod

from textual.app import App, ComposeResult
from textual.containers import Container, Vertical, Horizontal
from textual.widgets import Header, Footer, Input, RichLog, Button, Select
from textual.binding import Binding
from rich.markdown import Markdown


@dataclass
class Tool:
    """Represents a tool that the AI can call"""
    name: str
    description: str
    parameters: Dict[str, Any]
    function: Callable


class ToolRegistry:
    """Registry for managing available tools"""
    def __init__(self):
        self.tools: Dict[str, Tool] = {}

    def register(self, tool: Tool):
        """Register a new tool"""
        self.tools[tool.name] = tool

    def get_tool_definitions(self) -> List[Dict[str, Any]]:
        """Get tool definitions in the format expected by LLMs"""
        definitions = []
        for tool in self.tools.values():
            definitions.append({
                "type": "function",
                "function": {
                    "name": tool.name,
                    "description": tool.description,
                    "parameters": tool.parameters
                }
            })
        return definitions

    def execute_tool(self, name: str, arguments: Dict[str, Any]) -> Any:
        """Execute a tool by name with given arguments"""
        if name not in self.tools:
            raise ValueError(f"Tool {name} not found")
        return self.tools[name].function(**arguments)


class LLMProvider(ABC):
    """Abstract base class for LLM providers"""

    @abstractmethod
    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        """Send chat request and get response"""
        pass


class OpenAIProvider(LLMProvider):
    """OpenAI API provider"""

    def __init__(self, api_key: Optional[str] = None, model: str = "gpt-4-turbo-preview"):
        self.api_key = api_key or os.getenv("OPENAI_API_KEY")
        self.model = model
        try:
            from openai import OpenAI
            self.client = OpenAI(api_key=self.api_key)
        except ImportError:
            raise ImportError("openai package not installed. Run: pip install openai")

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        kwargs = {
            "model": self.model,
            "messages": messages
        }
        if tools:
            kwargs["tools"] = tools

        response = self.client.chat.completions.create(**kwargs)

        message = response.choices[0].message
        result = {
            "content": message.content,
            "tool_calls": []
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
            raise ImportError("anthropic package not installed. Run: pip install anthropic")

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        # Convert messages format
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
            # Convert OpenAI tool format to Anthropic format
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
            "tool_calls": []
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
            raise ImportError("requests package not installed. Run: pip install requests")

    def chat(self, messages: List[Dict], tools: Optional[List[Dict]] = None) -> Dict:
        payload = {
            "model": self.model,
            "messages": messages,
            "stream": False
        }

        if tools:
            payload["tools"] = tools

        response = self.requests.post(
            f"{self.base_url}/api/chat",
            json=payload
        )
        response.raise_for_status()

        data = response.json()
        message = data.get("message", {})

        result = {
            "content": message.get("content", ""),
            "tool_calls": []
        }

        if "tool_calls" in message:
            for tool_call in message["tool_calls"]:
                result["tool_calls"].append({
                    "id": tool_call.get("id", ""),
                    "name": tool_call["function"]["name"],
                    "arguments": tool_call["function"]["arguments"]
                })

        return result


class AgentTUI(App):
    """TUI application for AI Agent with tool calling"""

    CSS = """
    Screen {
        background: $surface;
    }

    #chat-container {
        height: 1fr;
        border: solid $primary;
    }

    #input-container {
        height: auto;
        padding: 1;
    }

    #provider-container {
        height: auto;
        padding: 1;
        background: $boost;
    }

    RichLog {
        background: $surface;
        color: $text;
        padding: 1;
    }

    Input {
        width: 1fr;
    }

    Button {
        margin-left: 1;
    }
    """

    BINDINGS = [
        Binding("ctrl+c", "quit", "Quit"),
        Binding("ctrl+l", "clear", "Clear Chat"),
    ]

    def __init__(self):
        super().__init__()
        self.tool_registry = ToolRegistry()
        self.messages: List[Dict] = []
        self.provider: Optional[LLMProvider] = None
        self.register_tools()

    def compose(self) -> ComposeResult:
        yield Header()

        with Vertical(id="provider-container"):
            with Horizontal():
                yield Select(
                    [
                        ("OpenAI (GPT-4)", "openai"),
                        ("Anthropic (Claude)", "anthropic"),
                        ("Ollama (Local)", "ollama"),
                    ],
                    prompt="Select Provider:",
                    id="provider-select"
                )
                yield Button("Connect", id="connect-btn", variant="primary")

        with Container(id="chat-container"):
            yield RichLog(id="chat-log", markup=True, wrap=True)

        with Horizontal(id="input-container"):
            yield Input(placeholder="Type your message...", id="message-input")
            yield Button("Send", id="send-btn", variant="primary")

        yield Footer()

    def register_tools(self):
        """Register available tools - override this to add custom tools"""
        # This will be extended with Obsidian tools
        pass

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "connect-btn":
            self.connect_provider()
        elif event.button.id == "send-btn":
            self.send_message()

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input.id == "message-input":
            self.send_message()

    def connect_provider(self):
        """Connect to selected LLM provider"""
        select = self.query_one("#provider-select", Select)
        provider_type = select.value

        chat_log = self.query_one("#chat-log", RichLog)

        try:
            if provider_type == "openai":
                self.provider = OpenAIProvider()
                chat_log.write("[bold green]✓ Connected to OpenAI[/bold green]")
            elif provider_type == "anthropic":
                self.provider = AnthropicProvider()
                chat_log.write("[bold green]✓ Connected to Anthropic Claude[/bold green]")
            elif provider_type == "ollama":
                self.provider = OllamaProvider()
                chat_log.write("[bold green]✓ Connected to Ollama (local)[/bold green]")
        except Exception as e:
            chat_log.write(f"[bold red]✗ Connection failed: {e}[/bold red]")

    def send_message(self):
        """Send user message and get AI response"""
        if not self.provider:
            chat_log = self.query_one("#chat-log", RichLog)
            chat_log.write("[bold yellow]⚠ Please connect to a provider first[/bold yellow]")
            return

        input_widget = self.query_one("#message-input", Input)
        user_message = input_widget.value.strip()

        if not user_message:
            return

        input_widget.value = ""
        chat_log = self.query_one("#chat-log", RichLog)

        # Display user message
        chat_log.write(f"[bold cyan]You:[/bold cyan] {user_message}")

        # Add to message history
        self.messages.append({
            "role": "user",
            "content": user_message
        })

        try:
            # Get AI response with tools
            tools = self.tool_registry.get_tool_definitions() if self.tool_registry.tools else None
            response = self.provider.chat(self.messages, tools=tools)

            # Handle tool calls
            if response["tool_calls"]:
                chat_log.write("[bold yellow]🔧 Executing tools...[/bold yellow]")

                for tool_call in response["tool_calls"]:
                    tool_name = tool_call["name"]
                    tool_args = tool_call["arguments"]

                    chat_log.write(f"[dim]  → {tool_name}({json.dumps(tool_args)})[/dim]")

                    try:
                        result = self.tool_registry.execute_tool(tool_name, tool_args)
                        chat_log.write(f"[dim]  ← {result}[/dim]")

                        # Add tool result to messages
                        self.messages.append({
                            "role": "assistant",
                            "content": response.get("content") or "",
                            "tool_calls": response["tool_calls"]
                        })
                        self.messages.append({
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "content": json.dumps(result)
                        })

                        # Get final response after tool execution
                        final_response = self.provider.chat(self.messages)
                        response = final_response
                    except Exception as e:
                        chat_log.write(f"[bold red]  ✗ Tool error: {e}[/bold red]")

            # Display AI response
            if response["content"]:
                chat_log.write(Markdown(f"**Assistant:** {response['content']}"))
                self.messages.append({
                    "role": "assistant",
                    "content": response["content"]
                })

        except Exception as e:
            chat_log.write(f"[bold red]✗ Error: {e}[/bold red]")

    def action_clear(self):
        """Clear chat history"""
        chat_log = self.query_one("#chat-log", RichLog)
        chat_log.clear()
        self.messages = []
        chat_log.write("[dim]Chat cleared[/dim]")


if __name__ == "__main__":
    app = AgentTUI()
    app.run()
