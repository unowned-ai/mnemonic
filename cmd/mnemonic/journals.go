package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/unowned-ai/mnemonic/pkg/db"
	"github.com/unowned-ai/mnemonic/pkg/memories"
)

var (
	dbPath  string
	walMode bool
	syncMode string
	activeOnly bool
)

var journalsCmd = &cobra.Command{
	Use:   "journals",
	Short: "Manage journals",
	Long:  `Create, list, update, and delete journals for storing memories.`,
}

var createJournalCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new journal",
	Long:  `Create a new journal with a name and optional description.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")

		if name == "" {
			return errors.New("journal name is required")
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		journal, err := memories.CreateJournal(context.Background(), dbConn, name, description)
		if err != nil {
			return fmt.Errorf("failed to create journal: %w", err)
		}

		printJournal(journal)
		return nil
	},
}

var getJournalCmd = &cobra.Command{
	Use:   "get [journal-id]",
	Short: "Get a journal by ID",
	Long:  `Retrieve a journal by its ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		journalIDStr := args[0]
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		journal, err := memories.GetJournal(context.Background(), dbConn, journalID)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to get journal: %w", err)
		}

		printJournal(journal)
		return nil
	},
}

var listJournalsCmd = &cobra.Command{
	Use:   "list",
	Short: "List journals",
	Long:  `List all journals, or only active ones.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		journals, err := memories.ListJournals(context.Background(), dbConn, activeOnly)
		if err != nil {
			return fmt.Errorf("failed to list journals: %w", err)
		}

		if len(journals) == 0 {
			fmt.Println("No journals found.")
			return nil
		}

		fmt.Println("Journals:")
		fmt.Println("ID | Name | Description | Active | Created At | Updated At")
		fmt.Println("------------------------------------------------------------")
		for _, j := range journals {
			createdAt := formatTimestamp(j.CreatedAt)
			updatedAt := formatTimestamp(j.UpdatedAt)
			fmt.Printf("%s | %s | %s | %t | %s | %s\n", 
				j.ID, j.Name, j.Description, j.Active, createdAt, updatedAt)
		}
		return nil
	},
}

var updateJournalCmd = &cobra.Command{
	Use:   "update [journal-id]",
	Short: "Update a journal",
	Long:  `Update a journal's name, description, or active status.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		journalIDStr := args[0]
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		active, _ := cmd.Flags().GetBool("active")

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		// First get the current journal to preserve any fields not being updated
		currentJournal, err := memories.GetJournal(context.Background(), dbConn, journalID)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to get journal: %w", err)
		}

		// Use existing values if no new values provided
		if name == "" {
			name = currentJournal.Name
		}
		if cmd.Flags().Changed("description") == false {
			description = currentJournal.Description
		}
		if cmd.Flags().Changed("active") == false {
			active = currentJournal.Active
		}

		journal, err := memories.UpdateJournal(context.Background(), dbConn, journalID, name, description, active)
		if err != nil {
			return fmt.Errorf("failed to update journal: %w", err)
		}

		fmt.Println("Journal updated successfully!")
		printJournal(journal)
		return nil
	},
}

var deleteJournalCmd = &cobra.Command{
	Use:   "delete [journal-id]",
	Short: "Delete a journal",
	Long:  `Permanently delete a journal by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		journalIDStr := args[0]
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		err = memories.DeleteJournal(context.Background(), dbConn, journalID)
		if errors.Is(err, memories.ErrJournalNotFound) {
			return fmt.Errorf("journal not found: %s", journalIDStr)
		}
		if err != nil {
			return fmt.Errorf("failed to delete journal: %w", err)
		}

		fmt.Printf("Journal %s deleted successfully!\n", journalIDStr)
		return nil
	},
}

var cleanJournalsCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete inactive journals",
	Long:  `Delete all inactive journals from the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		count, err := memories.DeleteInactiveJournals(context.Background(), dbConn)
		if err != nil {
			return fmt.Errorf("failed to clean inactive journals: %w", err)
		}

		fmt.Printf("Deleted %d inactive journals.\n", count)
		return nil
	},
}

func initJournalsCmd() {
	// Add common database flags
	journalsCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (required)")
	journalsCmd.PersistentFlags().BoolVar(&walMode, "wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode")
	journalsCmd.PersistentFlags().StringVar(&syncMode, "sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA)")
	journalsCmd.MarkPersistentFlagRequired("db")

	// Create command flags
	createJournalCmd.Flags().String("name", "", "Name of the journal (required)")
	createJournalCmd.Flags().String("description", "", "Description of the journal")
	createJournalCmd.MarkFlagRequired("name")

	// List command flags
	listJournalsCmd.Flags().BoolVar(&activeOnly, "active-only", false, "List only active journals")

	// Update command flags
	updateJournalCmd.Flags().String("name", "", "New name for the journal")
	updateJournalCmd.Flags().String("description", "", "New description for the journal")
	updateJournalCmd.Flags().Bool("active", true, "Set journal active status")

	// Add all commands to journals command
	journalsCmd.AddCommand(
		createJournalCmd,
		getJournalCmd,
		listJournalsCmd,
		updateJournalCmd,
		deleteJournalCmd,
		cleanJournalsCmd,
	)
}

func openDB() (*sql.DB, error) {
	if dbPath == "" {
		return nil, errors.New("database path is required")
	}
	return db.OpenDBConnection(dbPath, walMode, syncMode)
}

func printJournal(journal memories.Journal) {
	createdAt := formatTimestamp(journal.CreatedAt)
	updatedAt := formatTimestamp(journal.UpdatedAt)

	fmt.Println("Journal Details:")
	fmt.Printf("ID:          %s\n", journal.ID)
	fmt.Printf("Name:        %s\n", journal.Name)
	fmt.Printf("Description: %s\n", journal.Description)
	fmt.Printf("Active:      %t\n", journal.Active)
	fmt.Printf("Created At:  %s\n", createdAt)
	fmt.Printf("Updated At:  %s\n", updatedAt)
}

func formatTimestamp(timestamp float64) string {
	// Convert Unix timestamp to a human-readable format
	timeObj := time.Unix(int64(timestamp), 0)
	return timeObj.Format(time.RFC3339)
}