"""
Obsidian Tools - AI Agent tools for interacting with Obsidian vaults
These tools can be used with any AI agent that supports tool calling
"""

import os
import json
import re
from pathlib import Path
from typing import List, Dict, Any, Optional
from datetime import datetime


class ObsidianVault:
    """Interface to an Obsidian vault"""

    def __init__(self, vault_path: str):
        self.vault_path = Path(vault_path).expanduser().resolve()
        if not self.vault_path.exists():
            raise ValueError(f"Vault path does not exist: {vault_path}")

    def search_notes(self, query: str, case_sensitive: bool = False) -> List[Dict[str, str]]:
        """
        Search for notes containing a query string

        Args:
            query: Search term
            case_sensitive: Whether search should be case sensitive

        Returns:
            List of matching notes with file path and preview
        """
        results = []
        flags = 0 if case_sensitive else re.IGNORECASE

        for md_file in self.vault_path.rglob("*.md"):
            try:
                content = md_file.read_text(encoding='utf-8')
                if re.search(re.escape(query), content, flags):
                    # Get a preview of the match
                    lines = content.split('\n')
                    preview_lines = [line for line in lines if re.search(re.escape(query), line, flags)]
                    preview = '\n'.join(preview_lines[:3])

                    results.append({
                        "path": str(md_file.relative_to(self.vault_path)),
                        "title": md_file.stem,
                        "preview": preview[:200] + "..." if len(preview) > 200 else preview
                    })
            except Exception:
                continue

        return results

    def read_note(self, note_path: str) -> Dict[str, Any]:
        """
        Read the complete contents of a note

        Args:
            note_path: Path to note relative to vault root

        Returns:
            Note metadata and content
        """
        full_path = self.vault_path / note_path
        if not full_path.exists():
            raise FileNotFoundError(f"Note not found: {note_path}")

        content = full_path.read_text(encoding='utf-8')

        # Parse frontmatter if it exists
        frontmatter = {}
        main_content = content

        if content.startswith('---'):
            parts = content.split('---', 2)
            if len(parts) >= 3:
                try:
                    import yaml
                    frontmatter = yaml.safe_load(parts[1]) or {}
                    main_content = parts[2].strip()
                except:
                    # If yaml not available or parsing fails, skip frontmatter
                    pass

        return {
            "path": note_path,
            "title": full_path.stem,
            "content": main_content,
            "frontmatter": frontmatter,
            "modified": datetime.fromtimestamp(full_path.stat().st_mtime).isoformat()
        }

    def create_note(self, title: str, content: str, folder: str = "", tags: Optional[List[str]] = None) -> str:
        """
        Create a new note in the vault

        Args:
            title: Note title (will be used as filename)
            content: Note content
            folder: Subfolder within vault (optional)
            tags: List of tags to add (optional)

        Returns:
            Path to created note
        """
        # Sanitize filename
        filename = re.sub(r'[<>:"/\\|?*]', '', title)
        if not filename.endswith('.md'):
            filename += '.md'

        target_dir = self.vault_path / folder if folder else self.vault_path
        target_dir.mkdir(parents=True, exist_ok=True)

        file_path = target_dir / filename

        # Build frontmatter
        frontmatter_dict = {
            "created": datetime.now().isoformat(),
        }
        if tags:
            frontmatter_dict["tags"] = tags

        # Create content with frontmatter
        full_content = "---\n"
        for key, value in frontmatter_dict.items():
            if isinstance(value, list):
                full_content += f"{key}:\n"
                for item in value:
                    full_content += f"  - {item}\n"
            else:
                full_content += f"{key}: {value}\n"
        full_content += "---\n\n"
        full_content += content

        file_path.write_text(full_content, encoding='utf-8')

        return str(file_path.relative_to(self.vault_path))

    def update_note(self, note_path: str, content: str, append: bool = False) -> str:
        """
        Update an existing note

        Args:
            note_path: Path to note relative to vault root
            content: New content
            append: If True, append to existing content

        Returns:
            Confirmation message
        """
        full_path = self.vault_path / note_path
        if not full_path.exists():
            raise FileNotFoundError(f"Note not found: {note_path}")

        if append:
            existing = full_path.read_text(encoding='utf-8')
            content = existing + "\n\n" + content

        full_path.write_text(content, encoding='utf-8')
        return f"Updated note: {note_path}"

    def list_notes(self, folder: str = "", pattern: str = "*.md") -> List[Dict[str, str]]:
        """
        List all notes in the vault or a specific folder

        Args:
            folder: Subfolder to list (optional)
            pattern: Glob pattern for matching files

        Returns:
            List of notes with metadata
        """
        search_path = self.vault_path / folder if folder else self.vault_path
        notes = []

        for md_file in search_path.rglob(pattern):
            if md_file.is_file():
                notes.append({
                    "path": str(md_file.relative_to(self.vault_path)),
                    "title": md_file.stem,
                    "size": md_file.stat().st_size,
                    "modified": datetime.fromtimestamp(md_file.stat().st_mtime).isoformat()
                })

        return sorted(notes, key=lambda x: x["modified"], reverse=True)

    def get_backlinks(self, note_path: str) -> List[Dict[str, str]]:
        """
        Find all notes that link to the specified note

        Args:
            note_path: Path to note relative to vault root

        Returns:
            List of notes that contain links to this note
        """
        note_name = Path(note_path).stem
        backlinks = []

        # Common Obsidian link patterns
        patterns = [
            rf'\[\[{re.escape(note_name)}\]\]',  # [[Note]]
            rf'\[\[{re.escape(note_name)}\|.*?\]\]',  # [[Note|Alias]]
            rf'\[.*?\]\({re.escape(note_path)}\)',  # [text](path)
        ]

        for md_file in self.vault_path.rglob("*.md"):
            if md_file.stem == note_name:
                continue  # Skip the note itself

            try:
                content = md_file.read_text(encoding='utf-8')
                for pattern in patterns:
                    if re.search(pattern, content):
                        # Find the line with the link
                        lines = content.split('\n')
                        link_line = next((line for line in lines if re.search(pattern, line)), "")

                        backlinks.append({
                            "path": str(md_file.relative_to(self.vault_path)),
                            "title": md_file.stem,
                            "context": link_line.strip()[:200]
                        })
                        break  # Only add once per file
            except Exception:
                continue

        return backlinks

    def get_tags(self) -> Dict[str, int]:
        """
        Get all tags used in the vault with their frequencies

        Returns:
            Dictionary of tags and their counts
        """
        tags = {}

        for md_file in self.vault_path.rglob("*.md"):
            try:
                content = md_file.read_text(encoding='utf-8')

                # Find hashtags in content
                hashtags = re.findall(r'#([\w/\-]+)', content)
                for tag in hashtags:
                    tags[tag] = tags.get(tag, 0) + 1

                # Parse frontmatter tags
                if content.startswith('---'):
                    parts = content.split('---', 2)
                    if len(parts) >= 3:
                        frontmatter_section = parts[1]
                        # Simple YAML tags parsing
                        tag_matches = re.findall(r'tags:\s*\[(.*?)\]', frontmatter_section)
                        if tag_matches:
                            for tag_list in tag_matches:
                                for tag in tag_list.split(','):
                                    tag = tag.strip().strip('"\'')
                                    if tag:
                                        tags[tag] = tags.get(tag, 0) + 1

            except Exception:
                continue

        return dict(sorted(tags.items(), key=lambda x: x[1], reverse=True))


def get_obsidian_tool_definitions(vault_path: str) -> List[Dict[str, Any]]:
    """
    Get tool definitions for Obsidian operations in the format expected by LLMs

    Args:
        vault_path: Path to Obsidian vault

    Returns:
        List of tool definitions
    """
    vault = ObsidianVault(vault_path)

    tools = [
        {
            "type": "function",
            "function": {
                "name": "search_obsidian_notes",
                "description": "Search for notes in the Obsidian vault containing specific text",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "The search term to look for in notes"
                        },
                        "case_sensitive": {
                            "type": "boolean",
                            "description": "Whether the search should be case sensitive",
                            "default": False
                        }
                    },
                    "required": ["query"]
                }
            }
        },
        {
            "type": "function",
            "function": {
                "name": "read_obsidian_note",
                "description": "Read the complete contents of a specific note",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "note_path": {
                            "type": "string",
                            "description": "Path to the note relative to vault root (e.g., 'folder/note.md')"
                        }
                    },
                    "required": ["note_path"]
                }
            }
        },
        {
            "type": "function",
            "function": {
                "name": "create_obsidian_note",
                "description": "Create a new note in the Obsidian vault",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "title": {
                            "type": "string",
                            "description": "Title of the note (will be used as filename)"
                        },
                        "content": {
                            "type": "string",
                            "description": "Content of the note in Markdown format"
                        },
                        "folder": {
                            "type": "string",
                            "description": "Subfolder within vault (optional)",
                            "default": ""
                        },
                        "tags": {
                            "type": "array",
                            "items": {"type": "string"},
                            "description": "List of tags to add to the note",
                            "default": []
                        }
                    },
                    "required": ["title", "content"]
                }
            }
        },
        {
            "type": "function",
            "function": {
                "name": "update_obsidian_note",
                "description": "Update an existing note in the Obsidian vault",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "note_path": {
                            "type": "string",
                            "description": "Path to the note relative to vault root"
                        },
                        "content": {
                            "type": "string",
                            "description": "New content for the note"
                        },
                        "append": {
                            "type": "boolean",
                            "description": "If true, append to existing content instead of replacing",
                            "default": False
                        }
                    },
                    "required": ["note_path", "content"]
                }
            }
        },
        {
            "type": "function",
            "function": {
                "name": "list_obsidian_notes",
                "description": "List all notes in the vault or a specific folder",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "folder": {
                            "type": "string",
                            "description": "Subfolder to list (optional, empty for all notes)",
                            "default": ""
                        }
                    }
                }
            }
        },
        {
            "type": "function",
            "function": {
                "name": "get_obsidian_backlinks",
                "description": "Find all notes that link to a specific note",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "note_path": {
                            "type": "string",
                            "description": "Path to the note to find backlinks for"
                        }
                    },
                    "required": ["note_path"]
                }
            }
        },
        {
            "type": "function",
            "function": {
                "name": "get_obsidian_tags",
                "description": "Get all tags used in the vault with their frequencies",
                "parameters": {
                    "type": "object",
                    "properties": {}
                }
            }
        }
    ]

    return tools, vault


# Tool execution functions
def execute_obsidian_tool(vault: ObsidianVault, tool_name: str, arguments: Dict[str, Any]) -> Any:
    """Execute an Obsidian tool by name with given arguments"""

    tool_map = {
        "search_obsidian_notes": vault.search_notes,
        "read_obsidian_note": vault.read_note,
        "create_obsidian_note": vault.create_note,
        "update_obsidian_note": vault.update_note,
        "list_obsidian_notes": vault.list_notes,
        "get_obsidian_backlinks": vault.get_backlinks,
        "get_obsidian_tags": vault.get_tags,
    }

    if tool_name not in tool_map:
        raise ValueError(f"Unknown tool: {tool_name}")

    return tool_map[tool_name](**arguments)
