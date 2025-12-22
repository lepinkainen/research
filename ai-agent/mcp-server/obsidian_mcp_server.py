#!/usr/bin/env python3
"""
Obsidian MCP Server - Model Context Protocol server for Obsidian vault integration
Exposes Obsidian vault operations as MCP tools and resources
"""

import os
import sys
import json
import asyncio
from pathlib import Path
from typing import Any, Sequence

# Add shared directory to path
sys.path.insert(0, str(Path(__file__).parent.parent / "shared"))

from obsidian_tools import ObsidianVault

# MCP SDK imports
try:
    from mcp.server import Server
    from mcp.server.stdio import stdio_server
    from mcp.types import (
        Tool,
        TextContent,
        ImageContent,
        EmbeddedResource,
        Resource,
        ResourceTemplate,
    )
except ImportError:
    print("MCP SDK not installed. Install with: pip install mcp", file=sys.stderr)
    sys.exit(1)


class ObsidianMCPServer:
    """MCP Server for Obsidian vault operations"""

    def __init__(self, vault_path: str):
        self.vault = ObsidianVault(vault_path)
        self.server = Server("obsidian-mcp-server")

        # Register handlers
        self.setup_handlers()

    def setup_handlers(self):
        """Setup MCP protocol handlers"""

        @self.server.list_tools()
        async def list_tools() -> list[Tool]:
            """List available tools"""
            return [
                Tool(
                    name="search_notes",
                    description="Search for notes containing specific text",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "query": {
                                "type": "string",
                                "description": "Search term",
                            },
                            "case_sensitive": {
                                "type": "boolean",
                                "description": "Case sensitive search",
                                "default": False,
                            },
                        },
                        "required": ["query"],
                    },
                ),
                Tool(
                    name="read_note",
                    description="Read the complete contents of a note",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "note_path": {
                                "type": "string",
                                "description": "Path to note relative to vault root",
                            },
                        },
                        "required": ["note_path"],
                    },
                ),
                Tool(
                    name="create_note",
                    description="Create a new note in the vault",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "title": {
                                "type": "string",
                                "description": "Note title",
                            },
                            "content": {
                                "type": "string",
                                "description": "Note content in Markdown",
                            },
                            "folder": {
                                "type": "string",
                                "description": "Subfolder (optional)",
                                "default": "",
                            },
                            "tags": {
                                "type": "array",
                                "items": {"type": "string"},
                                "description": "Tags to add",
                                "default": [],
                            },
                        },
                        "required": ["title", "content"],
                    },
                ),
                Tool(
                    name="update_note",
                    description="Update an existing note",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "note_path": {
                                "type": "string",
                                "description": "Path to note",
                            },
                            "content": {
                                "type": "string",
                                "description": "New content",
                            },
                            "append": {
                                "type": "boolean",
                                "description": "Append instead of replace",
                                "default": False,
                            },
                        },
                        "required": ["note_path", "content"],
                    },
                ),
                Tool(
                    name="list_notes",
                    description="List all notes in vault or folder",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "folder": {
                                "type": "string",
                                "description": "Folder to list (optional)",
                                "default": "",
                            },
                        },
                    },
                ),
                Tool(
                    name="get_backlinks",
                    description="Find notes linking to a specific note",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "note_path": {
                                "type": "string",
                                "description": "Note path to find backlinks for",
                            },
                        },
                        "required": ["note_path"],
                    },
                ),
                Tool(
                    name="get_tags",
                    description="Get all tags with frequencies",
                    inputSchema={
                        "type": "object",
                        "properties": {},
                    },
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: Any) -> Sequence[TextContent]:
            """Execute a tool"""
            try:
                if name == "search_notes":
                    result = self.vault.search_notes(
                        arguments["query"],
                        arguments.get("case_sensitive", False)
                    )
                elif name == "read_note":
                    result = self.vault.read_note(arguments["note_path"])
                elif name == "create_note":
                    result = self.vault.create_note(
                        arguments["title"],
                        arguments["content"],
                        arguments.get("folder", ""),
                        arguments.get("tags", [])
                    )
                elif name == "update_note":
                    result = self.vault.update_note(
                        arguments["note_path"],
                        arguments["content"],
                        arguments.get("append", False)
                    )
                elif name == "list_notes":
                    result = self.vault.list_notes(arguments.get("folder", ""))
                elif name == "get_backlinks":
                    result = self.vault.get_backlinks(arguments["note_path"])
                elif name == "get_tags":
                    result = self.vault.get_tags()
                else:
                    raise ValueError(f"Unknown tool: {name}")

                return [TextContent(type="text", text=json.dumps(result, indent=2, default=str))]

            except Exception as e:
                return [TextContent(type="text", text=f"Error: {str(e)}")]

        @self.server.list_resources()
        async def list_resources() -> list[Resource]:
            """List available resources (recent notes)"""
            try:
                notes = self.vault.list_notes()[:10]  # Get 10 most recent
                resources = []

                for note in notes:
                    resources.append(
                        Resource(
                            uri=f"obsidian:///{note['path']}",
                            name=note['title'],
                            description=f"Note: {note['title']}",
                            mimeType="text/markdown",
                        )
                    )

                return resources
            except Exception as e:
                return []

        @self.server.read_resource()
        async def read_resource(uri: str) -> str:
            """Read a resource by URI"""
            if uri.startswith("obsidian:///"):
                path = uri[12:]  # Remove obsidian:/// prefix
                note = self.vault.read_note(path)
                return note["content"]
            raise ValueError(f"Unknown resource URI: {uri}")

    async def run(self):
        """Run the MCP server"""
        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options()
            )


async def main():
    """Main entry point"""
    vault_path = os.getenv("OBSIDIAN_VAULT_PATH")
    if not vault_path:
        vault_path = os.path.expanduser("~/Documents/Obsidian")

    if len(sys.argv) > 1:
        vault_path = sys.argv[1]

    try:
        server = ObsidianMCPServer(vault_path)
        await server.run()
    except Exception as e:
        print(f"Error starting MCP server: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
