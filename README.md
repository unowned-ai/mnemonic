# recall

A self-hostable datastore for memories to share with your AI models.

## Features

-   **Self-contained**: Single binary with no external dependencies
-   **SQLite-based**: Simple, reliable storage with automatic schema management
-   **MCP-compatible**: Works with Claude, Cursor, and other MCP-enabled AI tools
-   **Tagged memories**: Organize and retrieve your memories using flexible tagging
-   **User-friendly**: Smart defaults and automatic configuration

## Quick Start

### Installation

Download the latest binary for your platform from the releases page.

### Running the MCP Server

Simply run:

```bash
recall mcp
```

This starts the MCP server using the default database location for your platform:

-   Windows: `%USERPROFILE%\AppData\Roaming\recall\recall.db`
-   macOS: `~/Library/Application Support/recall/recall.db`
-   Linux: `~/.local/share/recall/recall.db`

You can specify a custom database path:

```bash
recall mcp --db ~/path/to/your/database.db
```

### Integrating with AI Tools

See [MCP Configuration Examples](docs/mcp-config-examples.md) for detailed setup instructions for:

-   Claude for Desktop
-   Cursor IDE
-   Other MCP-compatible tools

## Recent Enhancements

-   **Automatic Database Setup**: The MCP server now automatically initializes or upgrades the database schema on startup
-   **Smart Path Handling**: Paths with `~` are expanded to your home directory
-   **Directory Creation**: Parent directories are created automatically if they don't exist
-   **System-Specific Defaults**: Each platform has an appropriate default database location
-   **MCP Protocol Fix**: Database initialization logs now properly go to stderr instead of breaking JSON-RPC communication

## Modules

By default, Recall is built with all modules enabled. You can exclude specific modules using build tags:

-   **TUI Module**: A lightweight, keyboard-driven interface that lets you capture, browse, and preview your memories without leaving the terminal.
    -   Included by default
    -   To build without TUI: `go build -tags notui`
