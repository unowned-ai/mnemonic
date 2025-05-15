package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/unowned-ai/recall/pkg/memories"
)

const DefaultJournalName = "memory"

// RegisterPingTool registers a minimal health-check tool.
// This tool can be used to verify the Recall MCP server is alive and responsive,
// ensuring that the conversational memory and context management features are available.
func RegisterPingTool(s *server.MCPServer) {
	pingTool := mcp.NewTool(
		"ping",
		mcp.WithDescription("Responds with 'pong' to verify the Recall MCP server (your conversational memory and context assistant) is alive and ready to help."),
	)
	s.AddTool(pingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("pong_recall"), nil
	})
}

// RegisterCreateJournalTool registers the create_journal tool.
// Journals are fundamental for organizing your thoughts, project contexts, and conversation histories.
// Use this tool to create new journals to better structure and manage your recallable information.
func RegisterCreateJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"create_journal",
		mcp.WithDescription("Creates a new journal to organize your memories, project contexts, or conversation snippets. Essential for structuring your second brain."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name for the new journal. Choose a name that reflects the type of information you want to store, e.g., 'Project Alpha Notes', 'Daily Reflections', 'Meeting Summaries'.")),
		mcp.WithString("description", mcp.Description("Optional description for the journal. Use this to add more context about the journal's purpose or content.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.Params.Arguments["name"].(string)
		if strings.TrimSpace(name) == "" {
			return mcp.NewToolResultError("'name' parameter is required and must be non-empty"), nil
		}
		desc, _ := request.Params.Arguments["description"].(string)

		journal, err := memories.CreateJournal(ctx, db, name, desc)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create journal: %v", err)), nil
		}
		b, _ := json.Marshal(journal)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterListJournalsTool lists all journals (active & inactive).
// Useful for getting an overview of all your structured memory spaces.
// Call this to see where your information is organized and to decide where to store or retrieve context.
func RegisterListJournalsTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"list_journals",
		mcp.WithDescription("Lists all your available journals. Use this to see how your conversational memory is organized and to find specific context areas."),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journals, err := memories.ListJournals(ctx, db, false)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list journals: %v", err)), nil
		}
		if len(journals) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		b, _ := json.Marshal(journals)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterGetJournalTool retrieves a journal by name.
// Use this to get details about a specific journal, like its description, to understand its purpose for storing memories or context.
func RegisterGetJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"get_journal",
		mcp.WithDescription("Retrieves details for a specific journal by its name. Helps you understand the purpose and content of a particular memory or context space."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the journal to retrieve. This should be an existing journal name obtained from list_journals or a known journal for your context.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.Params.Arguments["name"].(string)
		if strings.TrimSpace(name) == "" {
			return mcp.NewToolResultError("'name' parameter is required"), nil
		}
		j, err := getJournalByName(ctx, db, name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal '%s': %v", name, err)), nil
		}
		if j == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", name)), nil
		}
		b, _ := json.Marshal(j)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterUpdateJournalTool updates journal metadata.
// Keep your memory organization up-to-date by renaming journals, updating their descriptions, or changing their active status.
// This helps in maintaining a clear and relevant structure for your contextual information.
func RegisterUpdateJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"update_journal",
		mcp.WithDescription("Updates an existing journal's name, description, or active status. Use this to refine the organization of your conversational memories and context."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Current name of the journal you want to update. This is your reference to the memory space.")),
		mcp.WithString("new_name", mcp.Description("Optional new name for the journal. Change this if the journal's purpose or content focus has evolved.")),
		mcp.WithString("description", mcp.Description("Optional new description. Update this to better reflect the kind of information stored within this memory space.")),
		mcp.WithBoolean("active", mcp.Description("Optional new active status (true/false). Set to false to archive a journal you no longer frequently access but want to keep for future reference.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.Params.Arguments["name"].(string)
		if strings.TrimSpace(name) == "" {
			return mcp.NewToolResultError("'name' parameter is required"), nil
		}
		currentJournal, err := getJournalByName(ctx, db, name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
		}
		if currentJournal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", name)), nil
		}
		newNameVal, _ := request.Params.Arguments["new_name"].(string)
		if strings.TrimSpace(newNameVal) == "" {
			newNameVal = currentJournal.Name
		}
		newDescVal, _ := request.Params.Arguments["description"].(string)
		if newDescVal == "" {
			newDescVal = currentJournal.Description
		}
		activeVal := currentJournal.Active
		if av, ok := request.Params.Arguments["active"].(bool); ok {
			activeVal = av
		}
		updated, err := memories.UpdateJournal(ctx, db, currentJournal.ID, newNameVal, newDescVal, activeVal)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update journal: %v", err)), nil
		}
		b, _ := json.Marshal(updated)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterDeleteJournalTool deletes a journal by name (except the default).
// Use this to remove journals that are no longer relevant, helping to keep your memory space clean and focused.
// Note: Deleting a journal also deletes all its entries (memories/context snippets).
func RegisterDeleteJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"delete_journal",
		mcp.WithDescription("Deletes a journal and all its associated entries. Use to remove outdated or irrelevant memory spaces. The default journal cannot be deleted."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the journal to delete. Be sure this is the correct journal as this action also removes all its stored memories.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := request.Params.Arguments["name"].(string)
		if strings.TrimSpace(name) == "" {
			return mcp.NewToolResultError("'name' parameter is required"), nil
		}
		if name == DefaultJournalName {
			return mcp.NewToolResultError(fmt.Sprintf("Deleting the default journal '%s' is not allowed", DefaultJournalName)), nil
		}
		j, err := getJournalByName(ctx, db, name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
		}
		if j == nil {
			return mcp.NewToolResultText(fmt.Sprintf("Journal '%s' not found, nothing to delete.", name)), nil
		}
		if err := memories.DeleteJournal(ctx, db, j.ID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete journal: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Journal '%s' deleted successfully.", name)), nil
	})
}

// entryWithTags embeds memories.Entry and adds a Tags slice for MCP responses.
type entryWithTags struct {
	memories.Entry
	Tags []string `json:"tags"`
}

// helper to convert an Entry to entryWithTags.
func enrichEntry(ctx context.Context, db *sql.DB, e memories.Entry) (entryWithTags, error) {
	var out entryWithTags
	out.Entry = e
	tagObjs, err := memories.ListTagsForEntry(ctx, db, e.ID)
	if err != nil {
		return out, err
	}
	for _, t := range tagObjs {
		out.Tags = append(out.Tags, t.Tag)
	}
	return out, nil
}

// RegisterCreateEntryTool registers the create_entry tool.
// This is a core function for populating your conversational memory.
// Use it frequently to save snippets of conversations, important facts, code examples, or any piece of context you want to recall later.
func RegisterCreateEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"create_entry",
		mcp.WithDescription("Creates a new entry (a piece of memory or context) within a journal. Your primary tool for saving information to your second brain."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name. Defaults to 'memory'. Specify a journal to organize this piece of information.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title for the new entry. Make it descriptive for easy recall, e.g., 'Key takeaways from API design meeting', 'Python snippet for S3 upload'.")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content for the new entry. This is the actual information you want to save and recall later.")),
		mcp.WithString("content_type", mcp.DefaultString("text/plain"), mcp.Description("Optional content type (e.g., 'text/markdown', 'application/json'). Helps in interpreting the content later.")),
		mcp.WithString("tags", mcp.Description("Optional comma-separated tags (e.g., 'project-x,bugfix,api'). Tags make your memories easily searchable and connect related pieces of context.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		title, _ := request.Params.Arguments["entry_title"].(string)
		content, _ := request.Params.Arguments["content"].(string)
		contentType, _ := request.Params.Arguments["content_type"].(string)
		tagsStr, _ := request.Params.Arguments["tags"].(string)
		if strings.TrimSpace(title) == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required"), nil
		}
		// Ensure journal exists (create if missing)
		journal, err := getJournalByName(ctx, db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error checking journal: %v", err)), nil
		}
		if journal == nil {
			journalPtr, errCreate := memories.CreateJournal(ctx, db, journalName, "")
			if errCreate != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to create journal '%s': %v", journalName, errCreate)), nil
			}
			journal = &journalPtr
		}
		entry, err := memories.CreateEntry(ctx, db, journal.ID, title, content, contentType)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create entry: %v", err)), nil
		}
		// Tagging if requested
		if tagsStr != "" {
			for _, t := range parseTags(tagsStr) {
				_ = memories.TagEntry(ctx, db, entry.ID, t) // Ignore individual tag errors for now
			}
		}
		enriched, _ := enrichEntry(ctx, db, entry)
		b, _ := json.Marshal(enriched)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterListEntriesTool registers list_entries (filter by journal or tags).
// Essential for retrieving stored memories and context.
// Use filters to narrow down your search and find the exact piece of information you need from your second brain.
func RegisterListEntriesTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"list_entries",
		mcp.WithDescription("Lists entries (memories/context snippets), optionally filtered by journal and/or tags. Your main tool for retrieving information from your conversational memory."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal filter. Specify a journal to list memories only from that context space.")),
		mcp.WithString("tags", mcp.Description("Optional comma-separated tags list (e.g., 'project-x,urgent'). Use tags to find specific or related pieces of information across your memories.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		tagsStr, _ := request.Params.Arguments["tags"].(string)
		tagsFilter := parseTags(tagsStr)

		var journals []memories.Journal
		if journalName != "" {
			j, err := getJournalByName(ctx, db, journalName)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
			}
			if j == nil {
				return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", journalName)), nil
			}
			journals = append(journals, *j)
		} else {
			list, err := memories.ListJournals(ctx, db, false)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error listing journals: %v", err)), nil
			}
			journals = list
		}

		var results []entryWithTags
		for _, j := range journals {
			es, err := memories.ListEntries(ctx, db, j.ID, false)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error listing entries: %v", err)), nil
			}
			for _, e := range es {
				if len(tagsFilter) == 0 {
					en, _ := enrichEntry(ctx, db, e)
					results = append(results, en)
					continue
				}
				entryTags, err := memories.ListTagsForEntry(ctx, db, e.ID)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Error fetching tags: %v", err)), nil
				}
				if hasAllTags(entryTags, tagsFilter) {
					en, _ := enrichEntry(ctx, db, e)
					results = append(results, en)
				}
			}
		}
		if len(results) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		b, _ := json.Marshal(results)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// Helper: check if entryTags include all desired tags.
func hasAllTags(entryTags []memories.Tag, desired []string) bool {
	if len(desired) == 0 {
		return true
	}
	tagSet := make(map[string]struct{}, len(entryTags))
	for _, t := range entryTags {
		tagSet[t.Tag] = struct{}{}
	}
	for _, d := range desired {
		if _, ok := tagSet[d]; !ok {
			return false
		}
	}
	return true
}

// RegisterGetEntryTool fetches entry by title.
// Allows you to retrieve a specific piece of memory or context when you know its title.
// Useful for focused recall of information.
func RegisterGetEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"get_entry",
		mcp.WithDescription("Retrieves a specific entry (memory/context) by its title. Use this for targeted recall when you know what you're looking for."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal. Specify if you know which memory space the entry resides in.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry. Must be an exact match to the title used when creating the memory.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		title, _ := request.Params.Arguments["entry_title"].(string)
		if strings.TrimSpace(title) == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required"), nil
		}
		journal, err := getJournalByName(ctx, db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", journalName)), nil
		}
		entry, err := getEntryByTitleAndJournalID(ctx, db, title, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving entry: %v", err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' not found", title)), nil
		}
		enriched, _ := enrichEntry(ctx, db, *entry)
		b, _ := json.Marshal(enriched)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterUpdateEntryTool updates an entry.
// Memories and context evolve. Use this tool to update existing entries with new information, titles, or content types.
// Keeps your recallable information accurate and current.
func RegisterUpdateEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"update_entry",
		mcp.WithDescription("Updates an existing entry (memory/context). Use this to modify or add to information you've previously saved, keeping your second brain up-to-date."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal. Helps locate the entry if it's not in the default 'memory' journal.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Current title of the entry you want to update. This is how you identify the memory to change.")),
		mcp.WithString("new_title", mcp.Description("Optional new title. Useful if the context of the memory has shifted or for better organization.")),
		mcp.WithString("new_content", mcp.Description("Optional new content. Update the core information of the memory here.")),
		mcp.WithString("new_content_type", mcp.Description("Optional new content type. Change if the format of the stored information has changed (e.g., from 'text/plain' to 'text/markdown'.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		title, _ := request.Params.Arguments["entry_title"].(string)
		journal, err := getJournalByName(ctx, db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", journalName)), nil
		}
		entry, err := getEntryByTitleAndJournalID(ctx, db, title, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving entry: %v", err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' not found", title)), nil
		}
		newTitle, _ := request.Params.Arguments["new_title"].(string)
		newContent, _ := request.Params.Arguments["new_content"].(string)
		newContentType, _ := request.Params.Arguments["new_content_type"].(string)

		updated, err := memories.UpdateEntry(ctx, db, entry.ID, newTitle, newContent, newContentType)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update entry: %v", err)), nil
		}
		enriched, _ := enrichEntry(ctx, db, updated)
		b, _ := json.Marshal(enriched)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterDeleteEntryTool deletes an entry by title.
// Remove memories or context snippets that are no longer needed or have become outdated.
// Helps keep your conversational memory focused and relevant.
func RegisterDeleteEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"delete_entry",
		mcp.WithDescription("Deletes an entry (memory/context) by its title. Use this to remove information that is no longer relevant from your second brain."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal. Specify to ensure you delete the entry from the correct memory space.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry to delete. Double-check the title to avoid accidental deletion of important memories.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		title, _ := request.Params.Arguments["entry_title"].(string)
		journal, err := getJournalByName(ctx, db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", journalName)), nil
		}
		entry, err := getEntryByTitleAndJournalID(ctx, db, title, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving entry: %v", err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultText(fmt.Sprintf("Entry '%s' not found, nothing to delete.", title)), nil
		}
		if err := memories.DeleteEntry(ctx, db, entry.ID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete entry: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Entry '%s' deleted successfully.", title)), nil
	})
}

// RegisterManageEntryTagsTool adds/removes tags for an entry.
// Tags are crucial for creating relationships between memories and enabling powerful, context-aware searches.
// Use this tool to refine the way your memories are interconnected and discovered.
func RegisterManageEntryTagsTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"manage_entry_tags",
		mcp.WithDescription("Adds or removes tags for a specific entry (memory/context). Tags are keywords that help you categorize, connect, and search your saved information effectively."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal. Helps locate the entry if it's not in the default 'memory' journal.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry whose tags you want to manage. Identifies the specific memory you're working with.")),
		mcp.WithString("add_tags", mcp.Description("Comma-separated tags to add (e.g., 'important,follow-up,idea'). Adding relevant tags improves the discoverability of this memory.")),
		mcp.WithString("remove_tags", mcp.Description("Comma-separated tags to remove. Removing tags can help if a memory's categorization has changed or to declutter search results.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		title, _ := request.Params.Arguments["entry_title"].(string)
		addStr, _ := request.Params.Arguments["add_tags"].(string)
		removeStr, _ := request.Params.Arguments["remove_tags"].(string)
		if addStr == "" && removeStr == "" {
			return mcp.NewToolResultError("At least one of 'add_tags' or 'remove_tags' must be provided."), nil
		}
		journal, err := getJournalByName(ctx, db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal: %v", err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found", journalName)), nil
		}
		entry, err := getEntryByTitleAndJournalID(ctx, db, title, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving entry: %v", err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' not found", title)), nil
		}
		for _, t := range parseTags(addStr) {
			_ = memories.TagEntry(ctx, db, entry.ID, t)
		}
		for _, t := range parseTags(removeStr) {
			_ = memories.DetachTag(ctx, db, entry.ID, t)
		}
		updatedEntry, _ := memories.GetEntry(ctx, db, entry.ID)
		enriched, _ := enrichEntry(ctx, db, updatedEntry)
		b, _ := json.Marshal(enriched)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterListTagsTool lists all distinct tags across the database.
// Provides an overview of all keywords used to categorize your memories.
// Useful for understanding your current tagging system and for finding relevant tags to use in searches or when creating new entries.
func RegisterListTagsTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"list_tags",
		mcp.WithDescription("Lists all unique tags currently stored in your conversational memory. Useful for discovering existing categories or finding relevant tags for searching or creating new memories."),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := db.QueryContext(ctx, "SELECT tag, created_at, updated_at FROM tags ORDER BY tag")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list tags: %v", err)), nil
		}
		defer rows.Close()
		var tags []memories.Tag
		for rows.Next() {
			var t memories.Tag
			if err := rows.Scan(&t.Tag, &t.CreatedAt, &t.UpdatedAt); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to scan tag: %v", err)), nil
			}
			tags = append(tags, t)
		}
		if len(tags) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		b, _ := json.Marshal(tags)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// RegisterSearchEntriesTool searches entries by tags across all journals.
// A powerful way to retrieve contextually related information by combining multiple tags.
// This allows for complex queries to find precisely the memories or context snippets you need.
func RegisterSearchEntriesTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"search_entries",
		mcp.WithDescription("Searches for entries (memories/context) matching ALL specified tags across all journals. A powerful tool to find interconnected pieces of information in your second brain."),
		mcp.WithString("tags", mcp.Required(), mcp.Description("Comma-separated list of tags (e.g., 'project-x,api,urgent'). Entries matching all these tags will be returned.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tagsStr, _ := request.Params.Arguments["tags"].(string)
		tagsFilter := parseTags(tagsStr)
		if len(tagsFilter) == 0 {
			return mcp.NewToolResultError("'tags' parameter is required and must be non-empty"), nil
		}
		journals, err := memories.ListJournals(ctx, db, false)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error listing journals: %v", err)), nil
		}
		var matched []entryWithTags
		for _, j := range journals {
			entries, err := memories.ListEntries(ctx, db, j.ID, false)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error listing entries: %v", err)), nil
			}
			for _, e := range entries {
				entryTags, err := memories.ListTagsForEntry(ctx, db, e.ID)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Error fetching tags: %v", err)), nil
				}
				if hasAllTags(entryTags, tagsFilter) {
					en, _ := enrichEntry(ctx, db, e)
					matched = append(matched, en)
				}
			}
		}
		if len(matched) == 0 {
			return mcp.NewToolResultText("[]"), nil
		}
		b, _ := json.Marshal(matched)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// parseTags splits a comma-separated tag list.
func parseTags(tagsStr string) []string {
	var result []string
	for _, t := range strings.Split(tagsStr, ",") {
		if v := strings.TrimSpace(t); v != "" {
			result = append(result, v)
		}
	}
	return result
}
