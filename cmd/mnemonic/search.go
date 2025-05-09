package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/unowned-ai/mnemonic/pkg/memories"
	// Assuming memories types will be used
)

var searchCmdJournalIDFlag string

var searchCmd = &cobra.Command{
	Use:   "search [tag1 tag2...]",
	Short: "Search entries by matching tags within a journal",
	Long:  `Search for entries in a specified journal based on a list of query tags. Entries are ranked by the number of matching tags.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return errors.New("requires at least one tag argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		queryTags := args

		if searchCmdJournalIDFlag == "" {
			// This should be caught by MarkFlagRequired, but good to double check.
			return errors.New("journal ID is required")
		}
		journalID, err := uuid.Parse(searchCmdJournalIDFlag)
		if err != nil {
			return fmt.Errorf("invalid journal ID: %w", err)
		}

		dbConn, err := openDB() // Assumes openDB() is accessible from this package (e.g. defined in journals.go)
		if err != nil {
			return err
		}
		defer dbConn.Close()

		results, err := memories.SearchEntriesByTagMatchSQL(cmd.Context(), dbConn, journalID, queryTags)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(results) == 0 {
			fmt.Println("No matching entries found.")
			return nil
		}

		fmt.Println("Matches | ID                                   | Title")
		fmt.Println("-----------------------------------------------------------------------") // Adjusted separator length
		for _, matchedEntry := range results {
			fmt.Printf("%-7d | %-36s | %s\n",
				matchedEntry.MatchCount,
				matchedEntry.Entry.ID.String(),
				matchedEntry.Entry.Title)
		}

		return nil
	},
}

func initSearchCmd() {
	searchCmd.Flags().StringVar(&searchCmdJournalIDFlag, "journal", "", "Journal ID to search within (required)")
	if err := searchCmd.MarkFlagRequired("journal"); err != nil {
		// This error typically happens at init time if the flag doesn't exist,
		// but cobra handles it. For robustness, one might log or panic here if critical.
		fmt.Fprintf(os.Stderr, "Error marking --journal flag required for search: %v\n", err)
		// os.Exit(1) // Or handle more gracefully depending on desired startup behavior
	}
	// No dbPath, walMode, syncMode flags here as they are persistent flags on a parent command (e.g. root or journalsCmd)
	// and use the package-level variables from journals.go or main.go
}
