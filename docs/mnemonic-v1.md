# recall

`recall` is a self-hostable datastore for shareable memories. The primary use case is to share memories with AI models while you are using them.
It is inspired by ChatGPT's powerful "memories" feature.

`recall` follows these design pillars:

1. `recall` is and always will be free, open source, and self hostable.

2. The user owns their memories, and they have full freedom in determining how these memories are used in the course of their conversations with their AI models.

3. `recall` memories can be tagged, with tags offering contextual cues as to how each memory can be used. In this way, `recall` allows for more directed use
   of stored memories than does ChatGPT.

4. Users can use their `recall` memories with any AI model on any platform that supports MCP tool calls.

## Roots

`recall` is a philosophical descendant of [`spire`](https://github.com/bugout-dev/spire), which is a tag-based knowledge management tool created by the team at
[Bugout.dev](https://bugout.dev). Spire proved the versatility and utility of storing data in tag-based ontologies. It is also difficult to use because of its
various service dependencie -- the Bugout.dev authentication and authorization service ([`brood`](https://github.com/bugout-dev/brood)), Postgres, and Elasticsearch, with
an optional Redis dependency.

`recall` aims to preserve the utility of `spire` without inheriting any of the friction that came with it. `recall` is built to be self-hostable first, with minimal
setup complexity.

## Who is `recall` for and how will they use it?

## Technical decisions

### Programming language

Go ... why

### Database

SQLite ... why

We'll use [`go-sqlite3`](https://github.com/mattn/go-sqlite3).

Custom migration framework for now. Keeps it simple and fast.

In the future, we may allow for different DB backends.

### MCP implementation

We'll use [`mcp-go`](https://github.com/mark3labs/mcp-go) and will only implement an stdio-transport server.

We won't worry about remote use case in v1, v1 is purely for people running `recall` locally. They can move their
sqlite file around to different machines as they need to.

### Client

We don't want to implement web client in V1. This decision will help us get validation faster because it means we
don't need to add a webserver to `recall` and also it will be easier for users to understand what the tool is doing.

We will implement a [TUI in bubbletea](https://github.com/charmbracelet/bubbletea).

---

## Planning

-   User journeys for `recall`. Who will use this and how?
-   Database management - do we want to use some external alembic-like tool or do we want to write our own db migration tool?
-   MCP server implementation - all MCP frameworks in Go are new/immature, so do we want to contribute to an existing framework or write our own?
-   Architecture for MCP - do we want to implement HTTP-based AND stdio-based MCP or only stdio-based and if users host remotely their stdio MCP can talk to their remote host
    over API.
-   Client. For V1 do we want a web frontend or text frontend enough? (e.g. [bubbletea](https://github.com/charmbracelet/bubbletea))

Cool ideas:

-   messages table that allows models to communicate with each other (email style)
