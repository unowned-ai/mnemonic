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
func RegisterPingTool(s *server.MCPServer) {
	pingTool := mcp.NewTool(
		"ping",
		mcp.WithDescription("Responds with 'pong' to verify the Recall MCP server is alive."),
	)
	s.AddTool(pingTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("pong_recall"), nil
	})
}

// RegisterCreateJournalTool registers the create_journal tool.
func RegisterCreateJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"create_journal",
		mcp.WithDescription("Creates a new journal."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name for the new journal.")),
		mcp.WithString("description", mcp.Description("Optional description for the journal.")),
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
func RegisterListJournalsTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"list_journals",
		mcp.WithDescription("Lists all available journals."),
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
func RegisterGetJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"get_journal",
		mcp.WithDescription("Retrieves details for a specific journal by its name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the journal to retrieve.")),
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
func RegisterUpdateJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"update_journal",
		mcp.WithDescription("Updates an existing journal's name, description, or active status."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Current name of the journal.")),
		mcp.WithString("new_name", mcp.Description("Optional new name for the journal.")),
		mcp.WithString("description", mcp.Description("Optional new description.")),
		mcp.WithBoolean("active", mcp.Description("Optional new active status (true/false).")),
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
func RegisterDeleteJournalTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"delete_journal",
		mcp.WithDescription("Deletes a journal and all its associated entries."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the journal to delete.")),
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
func RegisterCreateEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"create_entry",
		mcp.WithDescription("Creates a new entry within a journal."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title for the new entry.")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content for the new entry.")),
		mcp.WithString("content_type", mcp.DefaultString("text/plain"), mcp.Description("Optional content type.")),
		mcp.WithString("tags", mcp.Description("Optional comma-separated tags.")),
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
func RegisterListEntriesTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"list_entries",
		mcp.WithDescription("Lists entries, optionally filtered by journal and/or tags."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal filter.")),
		mcp.WithString("tags", mcp.Description("Optional comma-separated tags list.")),
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
func RegisterGetEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"get_entry",
		mcp.WithDescription("Retrieves entry details (including content and tags) by title."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry.")),
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
func RegisterUpdateEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"update_entry",
		mcp.WithDescription("Updates an existing entry."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Current title of the entry.")),
		mcp.WithString("new_title", mcp.Description("Optional new title.")),
		mcp.WithString("new_content", mcp.Description("Optional new content.")),
		mcp.WithString("new_content_type", mcp.Description("Optional new content type.")),
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
func RegisterDeleteEntryTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"delete_entry",
		mcp.WithDescription("Deletes an entry by title inside a journal."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry to delete.")),
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
func RegisterManageEntryTagsTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"manage_entry_tags",
		mcp.WithDescription("Adds or removes tags for a specific entry."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry.")),
		mcp.WithString("add_tags", mcp.Description("Comma-separated tags to add.")),
		mcp.WithString("remove_tags", mcp.Description("Comma-separated tags to remove.")),
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
func RegisterListTagsTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"list_tags",
		mcp.WithDescription("Lists all unique tags currently stored in the database."),
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
func RegisterSearchEntriesTool(s *server.MCPServer, db *sql.DB) {
	tool := mcp.NewTool(
		"search_entries",
		mcp.WithDescription("Searches for entries matching tags and/or full text across all journals."),
		mcp.WithString("tags", mcp.Description("Comma-separated list of tags.")),
		mcp.WithString("text", mcp.Description("Full text search query.")),
	)
	s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tagsStr, _ := request.Params.Arguments["tags"].(string)
		textQuery, _ := request.Params.Arguments["text"].(string)
		tagsFilter := parseTags(tagsStr)
		if len(tagsFilter) == 0 && strings.TrimSpace(textQuery) == "" {
			return mcp.NewToolResultError("provide 'tags' or 'text' parameter"), nil
		}
		journals, err := memories.ListJournals(ctx, db, false)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error listing journals: %v", err)), nil
		}
		var matched []entryWithTags
		for _, j := range journals {
			results, err := memories.SearchEntries(ctx, db, j.ID, tagsFilter, textQuery)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error searching entries: %v", err)), nil
			}
			for _, r := range results {
				en, _ := enrichEntry(ctx, db, r.Entry)
				matched = append(matched, en)
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
