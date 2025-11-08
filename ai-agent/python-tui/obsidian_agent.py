#!/usr/bin/env python3
"""
Obsidian AI Agent TUI - Specialized agent for Obsidian vault interaction
Extends the base agent with Obsidian-specific tools
"""

import sys
import os
from pathlib import Path

# Add shared directory to path
sys.path.insert(0, str(Path(__file__).parent.parent / "shared"))

from agent import AgentTUI, Tool, ToolRegistry
from obsidian_tools import ObsidianVault, execute_obsidian_tool, get_obsidian_tool_definitions


class ObsidianAgentTUI(AgentTUI):
    """AI Agent TUI specialized for Obsidian vault interaction"""

    def __init__(self, vault_path: str):
        super().__init__()
        self.vault = ObsidianVault(vault_path)

    def register_tools(self):
        """Register Obsidian-specific tools"""
        tool_defs, _ = get_obsidian_tool_definitions(str(self.vault.vault_path))

        # Register each Obsidian tool
        for tool_def in tool_defs:
            func = tool_def["function"]
            tool_name = func["name"]

            # Create a wrapper function for each tool
            def make_tool_function(name):
                def tool_function(**kwargs):
                    return execute_obsidian_tool(self.vault, name, kwargs)
                return tool_function

            tool = Tool(
                name=tool_name,
                description=func["description"],
                parameters=func["parameters"],
                function=make_tool_function(tool_name)
            )

            self.tool_registry.register(tool)

    def compose(self):
        """Override to show vault information"""
        result = super().compose()

        # Add system message about vault
        self.set_timer(0.1, self.show_vault_info)

        return result

    def show_vault_info(self):
        """Display vault information on startup"""
        from textual.widgets import RichLog
        chat_log = self.query_one("#chat-log", RichLog)
        chat_log.write(f"[bold green]📚 Obsidian Vault Loaded:[/bold green] {self.vault.vault_path}")
        chat_log.write(f"[dim]Available tools: {len(self.tool_registry.tools)}[/dim]")


def main():
    import argparse

    parser = argparse.ArgumentParser(description="Obsidian AI Agent TUI")
    parser.add_argument(
        "vault_path",
        nargs="?",
        default=os.getenv("OBSIDIAN_VAULT_PATH", "~/Documents/Obsidian"),
        help="Path to Obsidian vault (default: ~/Documents/Obsidian or OBSIDIAN_VAULT_PATH env var)"
    )

    args = parser.parse_args()

    try:
        app = ObsidianAgentTUI(args.vault_path)
        app.run()
    except ValueError as e:
        print(f"Error: {e}")
        print("\nUsage: python obsidian_agent.py [vault_path]")
        print("Or set OBSIDIAN_VAULT_PATH environment variable")
        sys.exit(1)


if __name__ == "__main__":
    main()
