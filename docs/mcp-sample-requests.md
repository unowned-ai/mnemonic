# MCP sample requests for smoke-testing `recall serve`

All messages are newline-delimited JSON. Each request **must** include a unique `id` field; responses echo that id.

> NOTE: The examples use the simplified MCP wire format supported by `mcp-go`. If you are driving the server with another client, adapt accordingly.

```jsonc
{
	"jsonrpc": "2.0",
	"id": 1,
	"method": "tools/call",
	"params": { "name": "ping" }
}
```

Expected result:

```jsonc
{
	"jsonrpc": "2.0",
	"id": 1,
	"result": { "type": "text", "content": "pong_recall" }
}
```
[{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"create_journal","arguments":{"name":"work","description":"Work notes"}}},{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"create_entry","arguments":{"journal_name":"work","entry_title":"todo-monday","content":"finish the report","tags":"tasks,urgent"}}},{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_entries","arguments":{"journal_name":"work"}}}]
---


## list tools

```jsonc
{"jsonrpc": "2.0","id": 1,"method": "tools/list"}
```

## End-to-end happy path

1. **Create a journal**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 2,
    	"method": "tools/call",
    	"params": {
    		"name": "create_journal",
    		"arguments": { "name": "work", "description": "Work notes" }
    	}
    }
    ```

2. **Create an entry**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 3,
    	"method": "tools/call",
    	"params": {
    		"name": "create_entry",
    		"arguments": {
    			"journal_name": "work",
    			"entry_title": "todo-monday",
    			"content": "finish the report",
    			"tags": "tasks,urgent"
    		}
    	}
    }
    ```

3. **List entries in that journal**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 4,
    	"method": "tools/call",
    	"params": {
    		"name": "list_entries",
    		"arguments": { "journal_name": "work" }
    	}
    }
    ```

4. **Get the entry**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 5,
    	"method": "tools/call",
    	"params": {
    		"name": "get_entry",
    		"arguments": {
    			"journal_name": "work",
    			"entry_title": "todo-monday"
    		}
    	}
    }
    ```

5. **Add a tag**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 6,
    	"method": "tools/call",
    	"params": {
    		"name": "manage_entry_tags",
    		"arguments": {
    			"journal_name": "work",
    			"entry_title": "todo-monday",
    			"add_tags": "office"
    		}
    	}
    }
    ```

6. **Search by tags**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 7,
    	"method": "tools/call",
    	"params": {
    		"name": "search_entries",
    		"arguments": { "tags": "tasks,office" }
    	}
    }
    ```

7. **Delete the entry**

    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 8,
    	"method": "tools/call",
    	"params": {
    		"name": "delete_entry",
    		"arguments": {
    			"journal_name": "work",
    			"entry_title": "todo-monday"
    		}
    	}
    }
    ```

8. **Delete the journal**
    ```jsonc
    {
    	"jsonrpc": "2.0",
    	"id": 9,
    	"method": "tools/call",
    	"params": { "name": "delete_journal", "arguments": { "name": "work" } }
    }
    ```

---

## How to run the smoke test

```bash
# in one terminal
recall --dbpath ./recall.db serve

# in another terminal (or using a tool like jq to pretty-print)
cat docs/mcp-sample-requests.ndjson | nc -U /tmp/recall.sock   # example if you have a unix-socket transport
```

Adjust transport to your environment (stdio / pipes, etc.).

---

Feel free to extend this list with edge-cases (duplicate entry creation, bad parameters, etc.).

cat docs/mcp-smoke.ndjson | ./recall mcp --db smoke.db
