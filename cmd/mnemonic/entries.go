package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/unowned-ai/mnemonic/pkg/memories"
)

var (
	journalIDFlag string
	contentTypeFlag string
	includeDeletedFlag bool
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

		if title == "" {
			return errors.New("entry title is required")
		}

		if content == "" {
			return errors.New("entry content is required")
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		entry, err := memories.CreateEntry(context.Background(), dbConn, journalID, title, content, contentTypeFlag)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDFlag)
		}
		if err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		printEntry(entry)
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

		printEntry(entry)
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
		fmt.Println("ID | Title | Content Type | Deleted | Created At | Updated At")
		fmt.Println("------------------------------------------------------------")
		for _, e := range entries {
			createdAt := formatTimestamp(e.CreatedAt)
			updatedAt := formatTimestamp(e.UpdatedAt)
			fmt.Printf("%s | %s | %s | %t | %s | %s\n", 
				e.ID, e.Title, e.ContentType, e.Deleted, createdAt, updatedAt)
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

		entry, err := memories.UpdateEntry(context.Background(), dbConn, entryID, title, content, contentTypeFlag)
		if errors.Is(err, memories.ErrEntryNotFound) {
			return fmt.Errorf("entry not found: %s", entryIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to update entry: %w", err)
		}

		fmt.Println("Entry updated successfully!")
		printEntry(entry)
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

		err = memories.DeleteEntry(context.Background(), dbConn, entryID)
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

		count, err := memories.CleanDeletedEntries(context.Background(), dbConn, journalID)
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

func initEntriesCmd() {
	// Common flags for entries commands
	entriesCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (required)")
	entriesCmd.PersistentFlags().BoolVar(&walMode, "wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode")
	entriesCmd.PersistentFlags().StringVar(&syncMode, "sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA)")
	entriesCmd.PersistentFlags().StringVar(&journalIDFlag, "journal", "", "Journal ID (required for most commands)")
	entriesCmd.PersistentFlags().StringVar(&contentTypeFlag, "content-type", "", "Content type (e.g., text/plain, text/markdown)")
	entriesCmd.MarkPersistentFlagRequired("db")

	// Create command flags
	createEntryCmd.Flags().String("title", "", "Title of the entry (required)")
	createEntryCmd.Flags().String("content", "", "Content of the entry (required)")
	createEntryCmd.MarkFlagRequired("title")
	createEntryCmd.MarkFlagRequired("content")
	createEntryCmd.MarkFlagRequired("journal")

	// List command flags
	listEntriesCmd.Flags().BoolVar(&includeDeletedFlag, "include-deleted", false, "Include soft-deleted entries in the listing")
	listEntriesCmd.MarkFlagRequired("journal")

	// Update command flags
	updateEntryCmd.Flags().String("title", "", "New title for the entry")
	updateEntryCmd.Flags().String("content", "", "New content for the entry")

	// Clean entries command flags
	cleanEntriesCmd.MarkFlagRequired("journal")

	// Add all commands to entries command
	entriesCmd.AddCommand(
		createEntryCmd,
		getEntryCmd,
		listEntriesCmd,
		updateEntryCmd,
		deleteEntryCmd,
		cleanEntriesCmd,
	)
}

func printEntry(entry memories.Entry) {
	createdAt := formatTimestamp(entry.CreatedAt)
	updatedAt := formatTimestamp(entry.UpdatedAt)

	fmt.Println("Entry Details:")
	fmt.Printf("ID:           %s\n", entry.ID)
	fmt.Printf("Journal ID:   %s\n", entry.JournalID)
	fmt.Printf("Title:        %s\n", entry.Title)
	fmt.Printf("Content Type: %s\n", entry.ContentType)
	fmt.Printf("Deleted:      %t\n", entry.Deleted)
	fmt.Printf("Created At:   %s\n", createdAt)
	fmt.Printf("Updated At:   %s\n", updatedAt)
	fmt.Println("\nContent:")
	fmt.Println("------------------------------------------------------------")
	fmt.Println(entry.Content)
	fmt.Println("------------------------------------------------------------")
}