package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/unowned-ai/recall/pkg/memories"
)

var (
	journalIDFlag      string
	contentTypeFlag    string
	includeDeletedFlag bool
	showTagsFlag       bool
)

var entriesCmd = &cobra.Command{
	Use:   "entries",
	Short: "Manage journal entries",
	Long:  `Create, list, update, and delete entries in journals.`,
}

var createEntryCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new entry in a journal",
	Long:  `Create a new entry with a title, content, and optional content type in a journal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		journalID, err := uuid.Parse(journalIDFlag)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		tagsStr, _ := cmd.Flags().GetString("tags")

		if title == "" {
			return errors.New("entry title is required")
		}

		if content == "" {
			return errors.New("entry content is required")
		}

		var tagNames []string
		if tagsStr != "" {
			tagNames = strings.Split(tagsStr, ",")
			for i, tag := range tagNames {
				tagNames[i] = strings.TrimSpace(tag)
			}
			actualTags := []string{}
			for _, tag := range tagNames {
				if tag != "" {
					actualTags = append(actualTags, tag)
				}
			}
			tagNames = actualTags
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		entry, err := memories.CreateEntry(cmd.Context(), dbConn, journalID, title, content, contentTypeFlag)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDFlag)
		}
		if err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		var lastTaggingError error
		for _, tagName := range tagNames {
			err = memories.TagEntry(cmd.Context(), dbConn, entry.ID, tagName)
			if err != nil {
				lastTaggingError = fmt.Errorf("failed to apply tag '%s': %w", tagName, err)
				cmd.PrintErrln(lastTaggingError)
			}
		}

		createdEntryTags, listTagsErr := memories.ListTagsForEntry(cmd.Context(), dbConn, entry.ID)
		if listTagsErr != nil {
			cmd.PrintErrf("Failed to retrieve tags for new entry: %v\n", listTagsErr)
		}
		printEntry(entry, createdEntryTags)

		if lastTaggingError != nil {
			return fmt.Errorf("entry created, but some tags failed to apply: %w", lastTaggingError)
		}
		return nil
	},
}

var getEntryCmd = &cobra.Command{
	Use:   "get [entry-id]",
	Short: "Get an entry by ID",
	Long:  `Retrieve an entry by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		entry, err := memories.GetEntry(context.Background(), dbConn, entryID)
		if errors.Is(err, memories.ErrEntryNotFound) {
			return fmt.Errorf("entry not found: %s", entryIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to get entry: %w", err)
		}

		var tags []memories.Tag
		if showTagsFlag {
			tags, err = memories.ListTagsForEntry(context.Background(), dbConn, entry.ID)
			if err != nil {
				return fmt.Errorf("failed to get tags for entry: %w", err)
			}
		}

		printEntry(entry, tags)
		return nil
	},
}

var listEntriesCmd = &cobra.Command{
	Use:   "list",
	Short: "List entries in a journal",
	Long:  `List all entries in a specified journal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		journalID, err := uuid.Parse(journalIDFlag)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		entries, err := memories.ListEntries(context.Background(), dbConn, journalID, includeDeletedFlag)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDFlag)
		}
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No entries found in this journal.")
			return nil
		}

		fmt.Println("Entries:")

		if showTagsFlag {
			fmt.Println("ID | Title | Content Type | Deleted | Tags | Created At | Updated At")
			fmt.Println("------------------------------------------------------------")
			for _, e := range entries {
				createdAt := formatTimestamp(e.CreatedAt)
				updatedAt := formatTimestamp(e.UpdatedAt)

				// Get tags for this entry
				tags, err := memories.ListTagsForEntry(context.Background(), dbConn, e.ID)
				if err != nil {
					return fmt.Errorf("failed to get tags for entry %s: %w", e.ID, err)
				}

				fmt.Printf("%s | %s | %s | %t | %s | %s | %s\n",
					e.ID, e.Title, e.ContentType, e.Deleted, formatTagsList(tags), createdAt, updatedAt)
			}
		} else {
			fmt.Println("ID | Title | Content Type | Deleted | Created At | Updated At")
			fmt.Println("------------------------------------------------------------")
			for _, e := range entries {
				createdAt := formatTimestamp(e.CreatedAt)
				updatedAt := formatTimestamp(e.UpdatedAt)
				fmt.Printf("%s | %s | %s | %t | %s | %s\n",
					e.ID, e.Title, e.ContentType, e.Deleted, createdAt, updatedAt)
			}
		}
		return nil
	},
}

var updateEntryCmd = &cobra.Command{
	Use:   "update [entry-id]",
	Short: "Update an entry",
	Long:  `Update an entry's title, content, or content type.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		entry, err := memories.UpdateEntry(cmd.Context(), dbConn, entryID, title, content, contentTypeFlag)
		if errors.Is(err, memories.ErrEntryNotFound) {
			return fmt.Errorf("entry not found: %s", entryIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to update entry: %w", err)
		}

		fmt.Println("Entry updated successfully!")
		var updatedEntryTags []memories.Tag
		updatedEntryTags, err = memories.ListTagsForEntry(cmd.Context(), dbConn, entry.ID)
		if err != nil {
			cmd.PrintErrf("Failed to retrieve tags for updated entry: %v\n", err)
		}
		printEntry(entry, updatedEntryTags)
		return nil
	},
}

var deleteEntryCmd = &cobra.Command{
	Use:   "delete [entry-id]",
	Short: "Soft delete an entry",
	Long:  `Mark an entry as deleted. The entry will still exist in the database but won't appear in listings unless you use the --include-deleted flag.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		err = memories.DeleteEntry(cmd.Context(), dbConn, entryID)
		if errors.Is(err, memories.ErrEntryNotFound) {
			return fmt.Errorf("entry not found: %s", entryIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to delete entry: %w", err)
		}

		fmt.Printf("Entry %s marked as deleted.\n", entryIDStr)
		return nil
	},
}

var cleanEntriesCmd = &cobra.Command{
	Use:   "clean",
	Short: "Permanently delete soft-deleted entries",
	Long:  `Permanently delete all entries that have been previously soft-deleted in a journal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		journalID, err := uuid.Parse(journalIDFlag)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		count, err := memories.CleanDeletedEntries(cmd.Context(), dbConn, journalID)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDFlag)
		}
		if err != nil {
			return fmt.Errorf("failed to clean deleted entries: %w", err)
		}

		fmt.Printf("Permanently deleted %d entries from journal %s.\n", count, journalIDFlag)
		return nil
	},
}

var tagEntryCmd = &cobra.Command{
	Use:   "tag [entry-id] [tag]...",
	Short: "Tag an entry",
	Long:  `Add one or more tags to an entry. Creates the tag if it doesn't exist.`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		tags := args[1:]

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		for _, tag := range tags {
			err = memories.TagEntry(context.Background(), dbConn, entryID, tag)
			if errors.Is(err, memories.ErrEntryNotFound) {
				return fmt.Errorf("entry not found: %s", entryIDStr)
			}
			if err != nil {
				return fmt.Errorf("failed to tag entry with '%s': %w", tag, err)
			}
		}

		fmt.Printf("Entry %s tagged with: %s\n", entryIDStr, strings.Join(tags, ", "))
		return nil
	},
}

var untagEntryCmd = &cobra.Command{
	Use:   "untag [entry-id] [tag]...",
	Short: "Remove tags from an entry",
	Long:  `Remove one or more tags from an entry.`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID: %w", err)
		}

		tags := args[1:]

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		var failedTags []string
		for _, tag := range tags {
			err = memories.DetachTag(context.Background(), dbConn, entryID, tag)
			if errors.Is(err, memories.ErrTagNotFound) {
				failedTags = append(failedTags, tag)
				continue
			}
			if errors.Is(err, memories.ErrEntryNotFound) {
				return fmt.Errorf("entry not found: %s", entryIDStr)
			}
			if err != nil {
				return fmt.Errorf("failed to remove tag '%s': %w", tag, err)
			}
		}

		if len(failedTags) == 0 {
			fmt.Printf("Tags removed from entry %s: %s\n", entryIDStr, strings.Join(tags, ", "))
		} else {
			fmt.Printf("Some tags were not found on entry %s: %s\n", entryIDStr, strings.Join(failedTags, ", "))
			if len(failedTags) < len(tags) {
				successTags := make([]string, 0, len(tags)-len(failedTags))
				for _, tag := range tags {
					found := false
					for _, failedTag := range failedTags {
						if tag == failedTag {
							found = true
							break
						}
					}
					if !found {
						successTags = append(successTags, tag)
					}
				}
				fmt.Printf("Successfully removed tags: %s\n", strings.Join(successTags, ", "))
			}
		}
		return nil
	},
}

func initEntriesCmd() {
	// entriesCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (required)") // Inherited from rootCmd
	// entriesCmd.PersistentFlags().BoolVar(&walMode, "wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode") // Inherited from rootCmd
	// entriesCmd.PersistentFlags().StringVar(&syncMode, "sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA)") // Inherited from rootCmd
	// entriesCmd.MarkPersistentFlagRequired("db") // Handled by openDB check or specific command needs

	entriesCmd.PersistentFlags().StringVar(&journalIDFlag, "journal", "", "Journal ID (required for most commands)")
	entriesCmd.PersistentFlags().StringVar(&contentTypeFlag, "content-type", "", "Content type (e.g., text/plain, text/markdown)")
	// entriesCmd.MarkPersistentFlagRequired("db") // This was already commented/removed implicitly

	createEntryCmd.Flags().String("title", "", "Title of the entry (required)")
	createEntryCmd.Flags().String("content", "", "Content of the entry (required)")
	createEntryCmd.Flags().String("tags", "", "Comma-separated list of tags for the entry")
	createEntryCmd.MarkFlagRequired("title")
	createEntryCmd.MarkFlagRequired("content")
	createEntryCmd.MarkFlagRequired("journal")

	getEntryCmd.Flags().BoolVar(&showTagsFlag, "tags", false, "Show tags for the entry")

	listEntriesCmd.Flags().BoolVar(&includeDeletedFlag, "include-deleted", false, "Include soft-deleted entries in the listing")
	listEntriesCmd.Flags().BoolVar(&showTagsFlag, "tags", false, "Show tags for each entry")
	listEntriesCmd.MarkFlagRequired("journal")

	updateEntryCmd.Flags().String("title", "", "New title for the entry")
	updateEntryCmd.Flags().String("content", "", "New content for the entry")

	cleanEntriesCmd.MarkFlagRequired("journal")

	entriesCmd.AddCommand(
		createEntryCmd,
		getEntryCmd,
		listEntriesCmd,
		updateEntryCmd,
		deleteEntryCmd,
		cleanEntriesCmd,
		tagEntryCmd,
		untagEntryCmd,
	)
}

func printEntry(entry memories.Entry, tags []memories.Tag) {
	createdAt := formatTimestamp(entry.CreatedAt)
	updatedAt := formatTimestamp(entry.UpdatedAt)

	fmt.Println("Entry Details:")
	fmt.Printf("ID:           %s\n", entry.ID)
	fmt.Printf("Journal ID:   %s\n", entry.JournalID)
	fmt.Printf("Title:        %s\n", entry.Title)
	fmt.Printf("Content Type: %s\n", entry.ContentType)
	fmt.Printf("Deleted:      %t\n", entry.Deleted)

	if len(tags) > 0 {
		fmt.Printf("Tags:         %s\n", formatTagsList(tags))
	}

	fmt.Printf("Created At:   %s\n", createdAt)
	fmt.Printf("Updated At:   %s\n", updatedAt)
	fmt.Println("\nContent:")
	fmt.Println("------------------------------------------------------------")
	fmt.Println(entry.Content)
	fmt.Println("------------------------------------------------------------")
}

func formatTagsList(tags []memories.Tag) string {
	if len(tags) == 0 {
		return "none"
	}

	tagNames := make([]string, len(tags))
	for i, tag := range tags {
		tagNames[i] = tag.Tag
	}

	return strings.Join(tagNames, ", ")
}
