package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	// "fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/unowned-ai/mnemonic/pkg/memories"
	// We might need access to the *sql.DB for other handlers, passed from MnemonicMCPServer
	// "database/sql"
)

const DefaultJournalName = "memory"

// RegisterPingTool registers the simple ping tool.
func RegisterPingTool(s *server.MCPServer) {
	pingTool := mcp.NewTool("ping",
		mcp.WithDescription("Responds with 'pong' to check if the Mnemonic MCP server is alive."),
		// No arguments needed for ping
	)
	s.AddTool(pingTool, pingHandler)
	// fmt.Println("Mnemonic Ping tool registered") // Optional: for server startup logging
}

// pingHandler is the simple handler for the ping tool.
func pingHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Ignore context and request parameters for a simple ping
	return mcp.NewToolResultText("pong_mnemonic"), nil // Differentiate from a generic pong
}

// RegisterCreateJournalTool registers the create_journal tool.
func RegisterCreateJournalTool(s *server.MCPServer, db *sql.DB) {
	createJournal := mcp.NewTool("create_journal",
		mcp.WithDescription("Creates a new journal."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name for the new journal.")),
		mcp.WithString("description", mcp.Description("Optional description for the journal.")),
	)
	s.AddTool(createJournal, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, nameOk := request.Params.Arguments["name"].(string)
		description, descOk := request.Params.Arguments["description"].(string)

		if !nameOk || name == "" {
			return mcp.NewToolResultError("'name' parameter is required and must be a non-empty string."), nil
		}

		// If description is not provided or not a string, use empty string (it's optional)
		if !descOk {
			description = ""
		}

		journal, err := memories.CreateJournal(db, name, description)
		if err != nil {
			// TODO: Check for specific errors, e.g., if journal name already exists (if we add that constraint)
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create journal: %v", err)), nil
		}

		// Return the created journal as JSON
		jsonResult, err := json.Marshal(journal)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize journal to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterListJournalsTool registers the list_journals tool.
func RegisterListJournalsTool(s *server.MCPServer, db *sql.DB) {
	listJournalsTool := mcp.NewTool("list_journals",
		mcp.WithDescription("Lists all available journals."),
		// No parameters for now
	)
	s.AddTool(listJournalsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journals, err := memories.ListJournals(db)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list journals: %v", err)), nil
		}

		if len(journals) == 0 {
			// Return empty list as JSON, or a specific message?
			// For consistency, let's return an empty JSON array.
			return mcp.NewToolResultText("[]"), nil
		}

		jsonResult, err := json.Marshal(journals)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize journals to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterGetJournalTool registers the get_journal tool.
func RegisterGetJournalTool(s *server.MCPServer, db *sql.DB) {
	getJournalTool := mcp.NewTool("get_journal",
		mcp.WithDescription("Retrieves details for a specific journal by its name."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the journal to retrieve.")),
	)
	s.AddTool(getJournalTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, nameOk := request.Params.Arguments["name"].(string)
		if !nameOk || name == "" {
			return mcp.NewToolResultError("'name' parameter is required and must be a non-empty string."), nil
		}

		journal, err := memories.GetJournalByName(db, name)
		if err != nil {
			// This could be sql.ErrNoRows from the underlying call if Scan failed, but GetJournalByName maps it to nil, nil
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving journal '%s': %v", name, err)), nil
		}

		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal with name '%s' not found.", name)), nil
		}

		jsonResult, err := json.Marshal(journal)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize journal to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterUpdateJournalTool registers the update_journal tool.
func RegisterUpdateJournalTool(s *server.MCPServer, db *sql.DB) {
	updateJournalTool := mcp.NewTool("update_journal",
		mcp.WithDescription("Updates an existing journal's name, description, or active status."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Current name of the journal to update.")),
		mcp.WithString("new_name", mcp.Description("Optional new name for the journal.")),
		mcp.WithString("description", mcp.Description("Optional new description for the journal.")),
		mcp.WithBoolean("active", mcp.Description("Optional new active status (true or false).")),
	)
	s.AddTool(updateJournalTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, nameOk := request.Params.Arguments["name"].(string)
		if !nameOk || name == "" {
			return mcp.NewToolResultError("Current 'name' parameter is required."), nil
		}

		// Check which fields are provided for update
		var newName *string
		if nn, ok := request.Params.Arguments["new_name"].(string); ok {
			if nn == "" {
				return mcp.NewToolResultError("'new_name' cannot be empty if provided."), nil
			}
			newName = &nn
		}

		var newDescription *string
		if nd, ok := request.Params.Arguments["description"].(string); ok {
			newDescription = &nd // Allow empty description
		}

		var newActive *bool
		if na, ok := request.Params.Arguments["active"].(bool); ok {
			newActive = &na
		}

		if newName == nil && newDescription == nil && newActive == nil {
			return mcp.NewToolResultError("No update fields provided (use new_name, description, or active)."), nil
		}

		// Find the journal by its current name to get its ID
		currentJournal, err := memories.GetJournalByName(db, name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s' to update: %v", name, err)), nil
		}
		if currentJournal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal with name '%s' not found.", name)), nil
		}

		// Call the underlying update function
		// Note: memories.UpdateJournal expects the ID, not the current name.
		updatedJournal, err := memories.UpdateJournal(db, currentJournal.ID, newName, newDescription, newActive)
		if err != nil {
			// Could be sql.ErrNoRows if ID somehow became invalid, or other DB error.
			// Also need to consider potential unique constraint violation if renaming to an existing name.
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update journal '%s': %v", name, err)), nil
		}

		if updatedJournal == nil {
			// This might happen if UpdateJournal returns nil, nil e.g. rowsAffected=0
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found during update process, or no change made.", name)), nil
		}

		jsonResult, err := json.Marshal(updatedJournal)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize updated journal to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterDeleteJournalTool registers the delete_journal tool.
func RegisterDeleteJournalTool(s *server.MCPServer, db *sql.DB) {
	deleteJournalTool := mcp.NewTool("delete_journal",
		mcp.WithDescription("Deletes a journal and all its associated entries."),
		mcp.WithString("name", mcp.Required(), mcp.Description("The name of the journal to delete.")),
	)
	s.AddTool(deleteJournalTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, nameOk := request.Params.Arguments["name"].(string)
		if !nameOk || name == "" {
			return mcp.NewToolResultError("'name' parameter is required."), nil
		}

		if name == DefaultJournalName {
			return mcp.NewToolResultError(fmt.Sprintf("Deleting the default journal '%s' is not allowed.", DefaultJournalName)), nil
		}

		// Find the journal by its name to get its ID
		journal, err := memories.GetJournalByName(db, name)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s' to delete: %v", name, err)), nil
		}
		if journal == nil {
			// If it doesn't exist, maybe that's success for deletion?
			// Let's return a message indicating it was not found.
			return mcp.NewToolResultText(fmt.Sprintf("Journal '%s' not found, nothing to delete.", name)), nil
		}

		// Call the underlying delete function
		err = memories.DeleteJournal(db, journal.ID)
		if err != nil {
			// This could be sql.ErrNoRows if the ID was somehow invalid after the GetByName check (unlikely)
			if err == sql.ErrNoRows {
				return mcp.NewToolResultText(fmt.Sprintf("Journal '%s' not found (or already deleted), nothing to delete.", name)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete journal '%s': %v", name, err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Journal '%s' deleted successfully.", name)), nil
	})
}

// RegisterCreateEntryTool registers the create_entry tool.
func RegisterCreateEntryTool(s *server.MCPServer, db *sql.DB) {
	createEntryTool := mcp.NewTool("create_entry",
		mcp.WithDescription("Creates a new entry within a journal."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name. Defaults to 'memory'.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title for the new entry.")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content for the new entry.")),
		mcp.WithString("content_type", mcp.DefaultString("text/plain"), mcp.Description("Optional content type (e.g., text/markdown).")),
		mcp.WithString("tags", mcp.Description("Optional comma-separated list of tags.")),
	)
	s.AddTool(createEntryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}

		entryTitle, titleOk := request.Params.Arguments["entry_title"].(string)
		content, contentOk := request.Params.Arguments["content"].(string)
		contentType, _ := request.Params.Arguments["content_type"].(string) // Default handled by MCP
		tagsStr, _ := request.Params.Arguments["tags"].(string)             // Optional

		if !titleOk || entryTitle == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required."), nil
		}
		if !contentOk { // Content itself can be empty, but param must exist
			return mcp.NewToolResultError("'content' parameter is required."), nil
		}

		// Ensure target journal exists (or create it)
		journal, err := memories.GetJournalByName(db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error checking for journal '%s': %v", journalName, err)), nil
		}
		if journal == nil {
			// Journal doesn't exist, create it (unless it's the default and creation failed somehow?)
			fmt.Printf("Journal '%s' not found, creating it.\n", journalName)
			journal, err = memories.CreateJournal(db, journalName, "") // Create with empty description
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to auto-create journal '%s': %v", journalName, err)), nil
			}
		}

		// TODO: Check if entry with this title already exists in this journal?
		// Current CreateEntry doesn't check, it just creates. Decide on desired behavior (error, update, allow duplicates?)
		// For now, proceed with creation.

		var tagsList []string
		if tagsStr != "" {
			tsl := strings.Split(tagsStr, ",")
			for _, tag := range tsl {
				t := strings.TrimSpace(tag)
				if t != "" {
					tagsList = append(tagsList, t)
				}
			}
		}

		entry, err := memories.CreateEntry(db, journal.ID, entryTitle, content, contentType, tagsList)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create entry '%s' in journal '%s': %v", entryTitle, journalName, err)), nil
		}

		jsonResult, err := json.Marshal(entry)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize created entry to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterListEntriesTool registers the list_entries tool.
func RegisterListEntriesTool(s *server.MCPServer, db *sql.DB) {
	listEntriesTool := mcp.NewTool("list_entries",
		mcp.WithDescription("Lists entries, optionally filtered by journal and/or tags."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name to filter by. Defaults to 'memory'.")),
		mcp.WithString("tags", mcp.Description("Optional comma-separated list of tags to filter entries by (entry must have all specified tags).")),
	)
	s.AddTool(listEntriesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		tagsStr, _ := request.Params.Arguments["tags"].(string)

		var journalID *uuid.UUID
		if journalName != "" {
			journal, err := memories.GetJournalByName(db, journalName)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s': %v", journalName, err)), nil
			}
			if journal == nil {
				return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found.", journalName)), nil
			}
			journalID = &journal.ID
		}

		var tagsList []string
		if tagsStr != "" {
			tsl := strings.Split(tagsStr, ",")
			for _, tag := range tsl {
				t := strings.TrimSpace(tag)
				if t != "" {
					tagsList = append(tagsList, t)
				}
			}
		}

		// Note: memories.ListEntries currently doesn't filter by entry title prefix from pathInfo.EntryTitle
		entries, err := memories.ListEntries(db, journalID, tagsList)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list entries: %v", err)), nil
		}

		if len(entries) == 0 {
			return mcp.NewToolResultText("[]"), nil // Return empty JSON array
		}

		jsonResult, err := json.Marshal(entries)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize entries to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterGetEntryTool registers the get_entry tool (replaces get_entry_contents from example).
func RegisterGetEntryTool(s *server.MCPServer, db *sql.DB) {
	getEntryTool := mcp.NewTool("get_entry",
		mcp.WithDescription("Retrieves the details (including content and tags) of a specific entry by its title inside a journal."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name. Defaults to 'memory'.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry to retrieve.")),
	)
	s.AddTool(getEntryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		entryTitle, titleOk := request.Params.Arguments["entry_title"].(string)
		if !titleOk || entryTitle == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required."), nil
		}

		journal, err := memories.GetJournalByName(db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s' for path '%s': %v", journalName, entryTitle, err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' specified in path '%s' not found.", journalName, entryTitle)), nil
		}

		entry, err := memories.GetEntryByTitleAndJournalID(db, entryTitle, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving entry '%s' from journal '%s': %v", entryTitle, journalName, err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry with title '%s' not found in journal '%s'.", entryTitle, journalName)), nil
		}

		jsonResult, err := json.Marshal(entry)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize entry to JSON: %v", err)), nil
		}
		// Optionally, could just return entry.Content if that's all the caller needs,
		// but returning the full entry gives more context (like tags, timestamps).
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterUpdateEntryTool registers the update_entry tool.
func RegisterUpdateEntryTool(s *server.MCPServer, db *sql.DB) {
	updateEntryTool := mcp.NewTool("update_entry",
		mcp.WithDescription("Updates an existing entry. Allows changing title, content, or content type."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name. Defaults to 'memory'.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry to update.")),
		mcp.WithString("new_title", mcp.Description("Optional new title for the entry.")),
		mcp.WithString("new_content", mcp.Description("Optional new content for the entry.")),
		mcp.WithString("new_content_type", mcp.Description("Optional new content type for the entry.")),
	)
	s.AddTool(updateEntryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		entryTitle, titleOk := request.Params.Arguments["entry_title"].(string)
		if !titleOk || entryTitle == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required."), nil
		}

		// Find the journal
		journal, err := memories.GetJournalByName(db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s' for path '%s': %v", journalName, entryTitle, err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' specified in path '%s' not found.", journalName, entryTitle)), nil
		}

		// Find the entry by current title and journal ID
		entry, err := memories.GetEntryByTitleAndJournalID(db, entryTitle, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error retrieving entry '%s' from journal '%s': %v", entryTitle, journalName, err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry with title '%s' not found in journal '%s'.", entryTitle, journalName)), nil
		}

		// Check which fields are provided for update
		var newTitle *string
		if nt, ok := request.Params.Arguments["new_title"].(string); ok {
			if nt == "" {
				return mcp.NewToolResultError("'new_title' cannot be empty if provided."), nil
			}
			newTitle = &nt
		}

		var newContent *string
		if nc, ok := request.Params.Arguments["new_content"].(string); ok {
			newContent = &nc // Allow empty content
		}

		var newContentType *string
		if nct, ok := request.Params.Arguments["new_content_type"].(string); ok {
			newContentType = &nct // Allow empty content type?
		}

		if newTitle == nil && newContent == nil && newContentType == nil {
			return mcp.NewToolResultError("No update fields provided (use new_title, new_content, or new_content_type)."), nil
		}

		// Call the underlying update function using the entry's ID
		updatedEntry, err := memories.UpdateEntry(db, entry.ID, newTitle, newContent, newContentType)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update entry '%s': %v", entryTitle, err)), nil
		}

		if updatedEntry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' not found during update process.", entryTitle)), nil
		}

		jsonResult, err := json.Marshal(updatedEntry)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize updated entry to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterDeleteEntryTool registers the delete_entry tool.
func RegisterDeleteEntryTool(s *server.MCPServer, db *sql.DB) {
	deleteEntryTool := mcp.NewTool("delete_entry",
		mcp.WithDescription("Deletes a specific entry by its title inside a journal."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name. Defaults to 'memory'.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry to delete.")),
	)
	s.AddTool(deleteEntryTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		entryTitle, titleOk := request.Params.Arguments["entry_title"].(string)
		if !titleOk || entryTitle == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required."), nil
		}

		// Find the journal
		journal, err := memories.GetJournalByName(db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s' for path '%s': %v", journalName, entryTitle, err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' specified in path '%s' not found.", journalName, entryTitle)), nil
		}

		// Find the entry to get its ID
		entry, err := memories.GetEntryByTitleAndJournalID(db, entryTitle, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding entry '%s' in journal '%s' to delete: %v", entryTitle, journalName, err)), nil
		}
		if entry == nil {
			// Entry doesn't exist, deletion is effectively successful/idempotent
			return mcp.NewToolResultText(fmt.Sprintf("Entry '%s' not found in journal '%s', nothing to delete.", entryTitle, journalName)), nil
		}

		// Call the underlying delete function
		err = memories.DeleteEntry(db, entry.ID)
		if err != nil {
			// Handle case where entry disappeared between check and delete?
			if err == sql.ErrNoRows {
				return mcp.NewToolResultText(fmt.Sprintf("Entry '%s' not found (or already deleted), nothing to delete.", entryTitle)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete entry '%s': %v", entryTitle, err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Entry '%s' deleted successfully from journal '%s'.", entryTitle, journalName)), nil
	})
}

// RegisterManageEntryTagsTool registers the manage_entry_tags tool.
func RegisterManageEntryTagsTool(s *server.MCPServer, db *sql.DB) {
	manageTagsTool := mcp.NewTool("manage_entry_tags",
		mcp.WithDescription("Adds or removes tags for a specific entry."),
		mcp.WithString("journal_name", mcp.DefaultString(DefaultJournalName), mcp.Description("Optional journal name. Defaults to 'memory'.")),
		mcp.WithString("entry_title", mcp.Required(), mcp.Description("Title of the entry whose tags will be managed.")),
		mcp.WithString("add_tags", mcp.Description("Comma-separated list of tags to add.")),
		mcp.WithString("remove_tags", mcp.Description("Comma-separated list of tags to remove.")),
	)
	s.AddTool(manageTagsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		journalName, _ := request.Params.Arguments["journal_name"].(string)
		if journalName == "" {
			journalName = DefaultJournalName
		}
		entryTitle, titleOk := request.Params.Arguments["entry_title"].(string)
		if !titleOk || entryTitle == "" {
			return mcp.NewToolResultError("'entry_title' parameter is required."), nil
		}

		addTagsStr, _ := request.Params.Arguments["add_tags"].(string)
		removeTagsStr, _ := request.Params.Arguments["remove_tags"].(string)

		if addTagsStr == "" && removeTagsStr == "" {
			return mcp.NewToolResultError("At least one of 'add_tags' or 'remove_tags' must be provided."), nil
		}

		// Find the journal and entry
		journal, err := memories.GetJournalByName(db, journalName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding journal '%s': %v", journalName, err)), nil
		}
		if journal == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Journal '%s' not found.", journalName)), nil
		}
		entry, err := memories.GetEntryByTitleAndJournalID(db, entryTitle, journal.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error finding entry '%s': %v", entryTitle, err)), nil
		}
		if entry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' not found in journal '%s'.", entryTitle, journalName)), nil
		}

		// Parse tag strings
		var tagsToAdd []string
		if addTagsStr != "" {
			tagsToAdd = parseTags(addTagsStr)
		}
		var tagsToRemove []string
		if removeTagsStr != "" {
			tagsToRemove = parseTags(removeTagsStr)
		}

		// Call the manage tags function
		err = memories.ManageEntryTags(db, entry.ID, tagsToAdd, tagsToRemove)
		if err != nil {
			if err == sql.ErrNoRows {
				// Should not happen if entry was found above, but handle defensively
				return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' not found during tag management.", entryTitle)), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to manage tags for entry '%s': %v", entryTitle, err)), nil
		}

		// Fetch the updated entry to return its current state
		updatedEntry, err := memories.GetEntryByID(db, entry.ID) // Use GetEntryByID as title might not be reliable if changed concurrently
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch updated entry details after tag management: %v", err)), nil
		}
		if updatedEntry == nil {
			return mcp.NewToolResultError(fmt.Sprintf("Entry '%s' could not be found after tag management.", entryTitle)), nil
		}

		jsonResult, err := json.Marshal(updatedEntry)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize updated entry to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// Helper function to parse comma-separated tag strings
func parseTags(tagsStr string) []string {
	var tagsList []string
	tsl := strings.Split(tagsStr, ",")
	for _, tag := range tsl {
		t := strings.TrimSpace(tag)
		if t != "" {
			tagsList = append(tagsList, t)
		}
	}
	return tagsList
}

// RegisterListTagsTool registers the list_tags tool.
func RegisterListTagsTool(s *server.MCPServer, db *sql.DB) {
	listTagsTool := mcp.NewTool("list_tags",
		mcp.WithDescription("Lists all unique tags currently stored in the database."),
		// No parameters needed
	)
	s.AddTool(listTagsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tags, err := memories.ListTags(db)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list tags: %v", err)), nil
		}

		if len(tags) == 0 {
			return mcp.NewToolResultText("[]"), nil // Return empty JSON array
		}

		jsonResult, err := json.Marshal(tags)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize tags to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}

// RegisterSearchEntriesTool registers the search_entries tool.
// Currently searches only by tags.
func RegisterSearchEntriesTool(s *server.MCPServer, db *sql.DB) {
	searchTool := mcp.NewTool("search_entries",
		mcp.WithDescription("Searches for entries matching all specified tags across all journals."),
		// TODO: Add support for full-text search query parameter?
		mcp.WithString("tags", mcp.Required(), mcp.Description("Comma-separated list of tags; entries must have all specified tags.")),
		// TODO: Add optional journal_name parameter to limit search scope?
	)
	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tagsStr, tagsOk := request.Params.Arguments["tags"].(string)
		if !tagsOk || tagsStr == "" {
			return mcp.NewToolResultError("'tags' parameter is required and must be non-empty."), nil
		}

		tagsList := parseTags(tagsStr)
		if len(tagsList) == 0 {
			return mcp.NewToolResultError("No valid tags provided in 'tags' parameter."), nil
		}

		// For now, search across all journals (journalID = nil)
		entries, err := memories.ListEntries(db, nil, tagsList)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to search entries by tags: %v", err)), nil
		}

		if len(entries) == 0 {
			return mcp.NewToolResultText("[]"), nil // Return empty JSON array
		}

		jsonResult, err := json.Marshal(entries)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to serialize search results to JSON: %v", err)), nil
		}
		return mcp.NewToolResultText(string(jsonResult)), nil
	})
}
