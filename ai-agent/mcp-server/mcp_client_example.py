#!/usr/bin/env python3
"""
Example MCP Client - Demonstrates how to connect to the Obsidian MCP Server
and use it with an AI model
"""

import asyncio
import os
from typing import Any

try:
    from mcp import ClientSession, StdioServerParameters
    from mcp.client.stdio import stdio_client
    from anthropic import Anthropic
except ImportError:
    print("Required packages not installed.")
    print("Install with: pip install mcp anthropic")
    exit(1)


class ObsidianMCPClient:
    """Client for interacting with Obsidian via MCP"""

    def __init__(self):
        self.session: ClientSession | None = None
        self.anthropic = Anthropic(api_key=os.getenv("ANTHROPIC_API_KEY"))

    async def connect(self, server_script_path: str, vault_path: str):
        """Connect to the MCP server"""
        server_params = StdioServerParameters(
            command="python",
            args=[server_script_path],
            env={"OBSIDIAN_VAULT_PATH": vault_path}
        )

        stdio_transport = await stdio_client(server_params)
        self.read_stream, self.write_stream = stdio_transport
        self.session = ClientSession(self.read_stream, self.write_stream)

        await self.session.initialize()

        # List available tools
        tools_result = await self.session.list_tools()
        print(f"Connected to MCP server with {len(tools_result.tools)} tools:")
        for tool in tools_result.tools:
            print(f"  - {tool.name}: {tool.description}")

    async def chat_with_tools(self, user_message: str):
        """Chat with Claude using MCP tools"""
        if not self.session:
            raise RuntimeError("Not connected to MCP server")

        # Get available tools
        tools_result = await self.session.list_tools()

        # Convert MCP tools to Anthropic format
        anthropic_tools = []
        for tool in tools_result.tools:
            anthropic_tools.append({
                "name": tool.name,
                "description": tool.description,
                "input_schema": tool.inputSchema
            })

        messages = [{"role": "user", "content": user_message}]

        print(f"\n🧑 You: {user_message}")

        # Chat loop with tool calling
        while True:
            response = self.anthropic.messages.create(
                model="claude-3-5-sonnet-20241022",
                max_tokens=4096,
                tools=anthropic_tools,
                messages=messages
            )

            # Process response
            tool_uses = []
            text_content = []

            for block in response.content:
                if block.type == "text":
                    text_content.append(block.text)
                elif block.type == "tool_use":
                    tool_uses.append(block)

            # Display text response
            if text_content:
                print(f"\n🤖 Claude: {' '.join(text_content)}")

            # If no tool calls, we're done
            if not tool_uses:
                break

            # Execute tools via MCP
            print(f"\n🔧 Executing {len(tool_uses)} tool(s)...")

            # Add assistant message with tool calls
            messages.append({
                "role": "assistant",
                "content": response.content
            })

            # Execute each tool and collect results
            tool_results = []
            for tool_use in tool_uses:
                print(f"   → {tool_use.name}({tool_use.input})")

                # Call tool via MCP
                result = await self.session.call_tool(
                    tool_use.name,
                    tool_use.input
                )

                tool_result_content = []
                for content in result.content:
                    if hasattr(content, 'text'):
                        tool_result_content.append({
                            "type": "text",
                            "text": content.text
                        })

                tool_results.append({
                    "type": "tool_result",
                    "tool_use_id": tool_use.id,
                    "content": tool_result_content
                })

                print(f"   ← Result: {content.text[:100]}...")

            # Add tool results to messages
            messages.append({
                "role": "user",
                "content": tool_results
            })

    async def close(self):
        """Close the connection"""
        if self.session:
            await self.session.__aexit__(None, None, None)


async def main():
    """Main demo"""
    print("=" * 60)
    print("Obsidian MCP Client Demo")
    print("=" * 60)

    # Configuration
    vault_path = os.getenv("OBSIDIAN_VAULT_PATH", os.path.expanduser("~/Documents/Obsidian"))
    server_script = os.path.join(os.path.dirname(__file__), "obsidian_mcp_server.py")

    client = ObsidianMCPClient()

    try:
        # Connect to MCP server
        print(f"\n📡 Connecting to MCP server...")
        print(f"   Vault: {vault_path}")
        await client.connect(server_script, vault_path)

        # Example conversations
        examples = [
            "List the 5 most recent notes in my vault",
            "Search for notes about 'Python'",
            "What tags are used most frequently in my vault?",
        ]

        print("\n" + "=" * 60)
        print("Example Queries:")
        print("=" * 60)

        for i, example in enumerate(examples, 1):
            print(f"\n[Example {i}/{len(examples)}]")
            await client.chat_with_tools(example)
            print("\n" + "-" * 60)

        # Interactive mode
        print("\n" + "=" * 60)
        print("Interactive Mode (Ctrl+C to exit)")
        print("=" * 60)

        while True:
            try:
                user_input = input("\n🧑 You: ").strip()
                if not user_input:
                    continue

                await client.chat_with_tools(user_input)

            except KeyboardInterrupt:
                print("\n\nExiting...")
                break

    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()

    finally:
        await client.close()


if __name__ == "__main__":
    asyncio.run(main())
