package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/unowned-ai/mnemonic/pkg/memories"
)

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags",
	Long:  `List and delete tags used in journals.`,
}

var listTagsCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tags in a journal",
	Long:  `List all tags used in a specific journal.`,
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

		tags, err := memories.ListTags(context.Background(), dbConn, journalID)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDFlag)
		}
		if err != nil {
			return fmt.Errorf("failed to list tags: %w", err)
		}

		if len(tags) == 0 {
			fmt.Println("No tags found in this journal.")
			return nil
		}

		fmt.Println("Tags:")
		fmt.Println("Tag | Created At | Updated At")
		fmt.Println("----------------------------------------")
		for _, t := range tags {
			createdAt := formatTimestamp(t.CreatedAt)
			updatedAt := formatTimestamp(t.UpdatedAt)
			fmt.Printf("%s | %s | %s\n", t.Tag, createdAt, updatedAt)
		}
		return nil
	},
}

var deleteTagCmd = &cobra.Command{
	Use:   "delete [tag-name]",
	Short: "Delete a tag",
	Long:  `Permanently delete a tag and remove it from all entries.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tagName := args[0]

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		err = memories.DeleteTag(context.Background(), dbConn, tagName)
		if errors.Is(err, memories.ErrTagNotFound) {
			return fmt.Errorf("tag not found: %s", tagName)
		}
		if err != nil {
			return fmt.Errorf("failed to delete tag: %w", err)
		}

		fmt.Printf("Tag '%s' deleted successfully!\n", tagName)
		return nil
	},
}

// Tag and untag commands are defined in entries.go

func initTagsCmd() {
	// tagsCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (required)") // Inherited from rootCmd
	// tagsCmd.PersistentFlags().BoolVar(&walMode, "wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode") // Inherited from rootCmd
	// tagsCmd.PersistentFlags().StringVar(&syncMode, "sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA)") // Inherited from rootCmd
	// tagsCmd.MarkPersistentFlagRequired("db") // Handled by openDB check

	listTagsCmd.Flags().StringVar(&journalIDFlag, "journal", "", "Journal ID (required)")
	listTagsCmd.MarkFlagRequired("journal")

	tagsCmd.AddCommand(
		listTagsCmd,
		deleteTagCmd,
	)
}

// formatTagsList is defined in entries.go
