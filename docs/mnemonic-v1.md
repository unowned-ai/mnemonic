# mnemonic

`mnemonic` is a self-hostable datastore for shareable memories. The primary use case is to share memories with AI models while you are using them.
It is inspired by ChatGPT's powerful "memories" feature.

`mnemonic` follows these design pillars:

1. `mnemonic` is and always will be free, open source, and self hostable.

2. The user owns their memories, and they have full freedom in determining how these memories are used in the course of their conversations with their AI models.

3. `mnemonic` memories can be tagged, with tags offering contextual cues as to how each memory can be used. In this way, `mnemonic` allows for more directed use
of stored memories than does ChatGPT.

4. Users can use their `mnemonic` memories with any AI model on any platform that supports MCP tool calls.

## Roots

`mnemonic` is a philosophical descendant of [`spire`](https://github.com/bugout-dev/spire), which is a tag-based knowledge management tool created by the team at
[Bugout.dev](https://bugout.dev). Spire proved the versatility and utility of storing data in tag-based ontologies. It is also difficult to use because of its
various service dependencie -- the Bugout.dev authentication and authorization service ([`brood`](https://github.com/bugout-dev/brood)), Postgres, and Elasticsearch, with
an optional Redis dependency.

`mnemonic` aims to preserve the utility of `spire` without inheriting any of the friction that came with it. `mnemonic` is built to be self-hostable first, with minimal
setup complexity.

