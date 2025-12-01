#!/usr/bin/env python3
"""
Context Restore Script

Reads and displays the project context from README.md, AGENTS.md, and .serena/memories/ folder.
This script helps quickly rebuild the mental model of a project.
"""

import os
import sys
import json
from pathlib import Path
from typing import Optional


def find_agents_md(project_root: Path) -> Optional[Path]:
    """Locate AGENTS.md in the project root."""
    agents_md = project_root / "AGENTS.md"

    if agents_md.exists():
        return agents_md

    return None


def find_context_directory(project_root: Path) -> Optional[Path]:
    """Locate .serena/memories directory in the project root."""
    context_dir = project_root / ".serena/memories"

    if context_dir.exists() and context_dir.is_dir():
        return context_dir

    return None


def find_readme(project_root: Path) -> Optional[Path]:
    """Locate README.md in the project root."""
    readme = project_root / "README.md"

    if readme.exists():
        return readme

    return None


def load_metadata(context_dir: Path) -> Optional[dict]:
    """Load metadata from .serena/memories/metadata.json if it exists."""
    metadata_file = context_dir / "metadata.json"

    if not metadata_file.exists():
        return None

    try:
        with open(metadata_file, 'r') as f:
            return json.load(f)
    except Exception as e:
        print(f"⚠️  Warning: Could not load metadata: {e}")
        return None


def list_context_files(context_dir: Path) -> list:
    """List all files in the .serena/memories directory."""
    files = []

    for item in context_dir.iterdir():
        if item.is_file() and item.name not in ['.gitkeep', 'metadata.json']:
            files.append(item)

    return sorted(files)


def display_context_summary(agents_md: Path, readme: Optional[Path], context_dir: Optional[Path], metadata: Optional[dict]):
    """Display a summary of the available context."""

    print("\n" + "="*70)
    print("📋 PROJECT CONTEXT SUMMARY")
    print("="*70)

    # Display metadata if available
    if metadata:
        print(f"\n📅 Context saved: {metadata.get('created_at', 'Unknown')}")
        print(f"📁 Project root: {metadata.get('project_root', 'Unknown')}")

    # Display README.md info
    if readme:
        print(f"\n📖 README: {readme}")
        file_size = readme.stat().st_size
        print(f"   Size: {file_size:,} bytes")

    # Display AGENTS.md info
    print(f"\n📄 Main context document: {agents_md}")
    file_size = agents_md.stat().st_size
    print(f"   Size: {file_size:,} bytes")

    # Display .serena/memories directory info
    if context_dir:
        context_files = list_context_files(context_dir)
        print(f"\n📂 Context directory: {context_dir}")
        print(f"   Reference documents: {len(context_files)}")

        if context_files:
            print("\n   Available references:")
            for file in context_files:
                file_size = file.stat().st_size
                print(f"   - {file.name} ({file_size:,} bytes)")
    else:
        print("\n📂 No .serena/memories directory found")

    print("\n" + "="*70)


def read_agents_md(agents_md: Path) -> str:
    """Read the contents of AGENTS.md."""
    with open(agents_md, 'r') as f:
        return f.read()


def main():
    """Main execution function."""

    # Determine project root (current working directory)
    project_root = Path.cwd()

    print(f"🔍 Searching for context in: {project_root}")

    # Find AGENTS.md
    agents_md = find_agents_md(project_root)

    if not agents_md:
        print("\n❌ Error: AGENTS.md not found in project root")
        print("\nTo create context documentation, run:")
        print("  python context_save.py")
        return 1

    # Find README.md
    readme = find_readme(project_root)

    # Find .serena/memories directory
    context_dir = find_context_directory(project_root)

    # Load metadata
    metadata = None
    if context_dir:
        metadata = load_metadata(context_dir)

    # Display summary
    display_context_summary(agents_md, readme, context_dir, metadata)

    # Automatically display README.md content if it exists
    if readme:
        print("\n" + "="*70)
        print("README.MD CONTENT")
        print("="*70 + "\n")

        with open(readme, 'r') as f:
            print(f.read())

        print("\n" + "="*70)

    # Automatically display AGENTS.md content
    print("\n" + "="*70)
    print("AGENTS.MD CONTENT")
    print("="*70 + "\n")

    content = read_agents_md(agents_md)
    print(content)

    print("\n" + "="*70)

    # Automatically display all reference documents
    if context_dir:
        context_files = list_context_files(context_dir)

        if context_files:
            for file in context_files:
                print("\n" + "="*70)
                print(f"REFERENCE: {file.name}")
                print("="*70 + "\n")

                with open(file, 'r') as f:
                    print(f.read())

                print("\n" + "="*70)

    print("\n✨ Context restore complete!")
    print(f"\n💡 All context documentation has been loaded")

    return 0


if __name__ == "__main__":
    sys.exit(main())