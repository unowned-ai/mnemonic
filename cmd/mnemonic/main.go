package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	mnemonic "github.com/unowned-ai/mnemonic/pkg"
	pkgdb "github.com/unowned-ai/mnemonic/pkg/db"
	"github.com/unowned-ai/mnemonic/pkg/memories"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var dbPath string

var rootCmd = &cobra.Command{
	Use:     "mnemonic",
	Short:   "A self-hostable datastore for your memories to share with your AI models.",
	Long:    ``,
	Version: fmt.Sprintf("v%s", mnemonic.Version),
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var completionShells = []string{"bash", "zsh", "fish", "powershell"}

var completionCmd = &cobra.Command{
	Use:   fmt.Sprintf("completion %s", strings.Join(completionShells, "|")),
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for mnemonic.

The command prints a completion script to stdout. You can source it in your shell
or install it to the appropriate location for your shell to enable completions permanently.

Examples:

  Bash (current shell):
    $ source <(mnemonic completion bash)

  Bash (persist):
    $ mnemonic completion bash > /etc/bash_completion.d/mnemonic

  Zsh:
    $ mnemonic completion zsh > "${fpath[1]}/_mnemonic"

  Fish:
    $ mnemonic completion fish | source
    $ mnemonic completion fish > ~/.config/fish/completions/mnemonic.fish

  PowerShell:
    PS> mnemonic completion powershell | Out-String | Invoke-Expression`,
	DisableFlagsInUseLine: true,
	ValidArgs:             completionShells,
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return rootCmd.GenPowerShellCompletion(cmd.OutOrStdout())
		default:
			return fmt.Errorf("unsupported shell: %s", args[0])
		}
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of mnemonic",
	Long:  `All software has versions. This is mnemonic's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(mnemonic.Version)
	},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage the Mnemonic database",
	Long:  `Provides commands for managing the Mnemonic SQLite database, including schema upgrades. GIGO.`,
}

var dbUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the Mnemonic database schema to the latest version for the memoriesdb component",
	Long:  `Connects to the SQLite database at the specified path (via --dbpath) and applies any necessary\\nschema migrations to bring the memoriesdb component up to the current application schema version. \\nIf the database does not exist or is uninitialized for this component, it will be created \\nand initialized with the latest schema for the memoriesdb component.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		walEnabled, _ := cmd.Flags().GetBool("wal")
		syncMode, _ := cmd.Flags().GetString("sync")

		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		fmt.Printf("Attempting to upgrade memoriesdb component in database at: %s (WAL: %t, Sync: %s)\\n", dbPath, walEnabled, syncMode)

		dbConn, err := pkgdb.OpenDBConnection(dbPath, walEnabled, syncMode)
		if err != nil {
			return err
		}
		defer dbConn.Close()

		if err := pkgdb.UpgradeDB(dbConn, dbPath, pkgdb.TargetSchemaVersion); err != nil {
			return err
		}
		return nil
	},
}

var journalsCmd = &cobra.Command{
	Use:   "journals",
	Short: "Manage journals",
	Long:  `Provides commands for creating, listing, getting, updating, and deleting journals.`,
}

var journalCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new journal",
	Long:  `Creates a new journal with the given name and optional description.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")

		if name == "" {
			return fmt.Errorf("journal name cannot be empty")
		}
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		journal, err := memories.CreateJournal(dbConn, name, description)
		if err != nil {
			return fmt.Errorf("failed to create journal: %w", err)
		}

		fmt.Println("Journal created successfully:")
		output, err := json.MarshalIndent(journal, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format journal output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var journalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all journals",
	Long:  `Lists all journals currently stored in the database.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL") // Assuming WAL and NORMAL sync
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		journals, err := memories.ListJournals(dbConn)
		if err != nil {
			return fmt.Errorf("failed to list journals: %w", err)
		}

		if len(journals) == 0 {
			fmt.Println("No journals found.")
			return nil
		}

		fmt.Println("Journals:")
		output, err := json.MarshalIndent(journals, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format journals output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var journalGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a specific journal by its ID",
	Long:  `Retrieves and displays the details of a specific journal using its UUID.`,
	Args:  cobra.ExactArgs(1), // Requires exactly one argument: the ID
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		journalIDStr := args[0]
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal ID format: %w", err)
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL") // Assuming WAL and NORMAL sync
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		journal, err := memories.GetJournalByID(dbConn, journalID)
		if err != nil {
			// This includes sql.ErrNoRows if the handler doesn't wrap it
			return fmt.Errorf("failed to get journal: %w", err)
		}

		if journal == nil {
			fmt.Printf("Journal with ID %s not found.\n", journalIDStr)
			return nil
		}

		fmt.Printf("Journal (ID: %s):\n", journalIDStr)
		output, err := json.MarshalIndent(journal, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format journal output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var journalUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an existing journal",
	Long:  `Updates an existing journal with the given ID. Only provided fields will be updated.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		journalIDStr := args[0]
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal ID format: %w", err)
		}

		var name, description *string
		var active *bool

		if cmd.Flags().Changed("name") {
			n, _ := cmd.Flags().GetString("name")
			name = &n
		}
		if cmd.Flags().Changed("description") {
			d, _ := cmd.Flags().GetString("description")
			description = &d
		}

		activeFlagSet := cmd.Flags().Changed("active")
		inactiveFlagSet := cmd.Flags().Changed("inactive")

		if activeFlagSet && inactiveFlagSet {
			return fmt.Errorf("cannot use --active and --inactive flags simultaneously")
		}
		if activeFlagSet {
			a := true
			active = &a
		} else if inactiveFlagSet {
			a := false
			active = &a
		}

		if name == nil && description == nil && active == nil {
			fmt.Println("No update fields provided. Use --name, --description, --active, or --inactive.")
			// Optionally, just fetch and display the current journal
			// For now, we require at least one change to be specified.
			return nil
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		updatedJournal, err := memories.UpdateJournal(dbConn, journalID, name, description, active)
		if err != nil {
			return fmt.Errorf("failed to update journal: %w", err)
		}

		if updatedJournal == nil {
			// This case also handles if UpdateJournal returned nil, nil due to RowsAffected == 0
			fmt.Printf("Journal with ID %s not found or no update performed.\n", journalIDStr)
			return nil
		}

		fmt.Println("Journal updated successfully:")
		output, err := json.MarshalIndent(updatedJournal, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format journal output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var journalDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a journal by its ID",
	Long:  `Deletes a specific journal and all its associated entries from the database using its UUID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		journalIDStr := args[0]
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal ID format: %w", err)
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL") // Assuming WAL and NORMAL sync
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		err = memories.DeleteJournal(dbConn, journalID)
		if err != nil {
			if err == sql.ErrNoRows { // Imported "database/sql"
				fmt.Printf("Journal with ID %s not found.\n", journalIDStr)
				return nil // Not an error for the CLI if not found, just a message
			}
			return fmt.Errorf("failed to delete journal: %w", err)
		}

		fmt.Printf("Journal with ID %s and its associated entries deleted successfully.\n", journalIDStr)
		return nil
	},
}

// New command for entries
var entriesCmd = &cobra.Command{
	Use:   "entries",
	Short: "Manage entries within journals",
	Long:  `Provides commands for creating, listing, getting, updating, deleting, and tagging entries.`,
}

var entryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new entry in a journal",
	Long:  `Creates a new entry with the given title, content, and optional tags, associated with a specific journal.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		journalIDStr, _ := cmd.Flags().GetString("journal-id")
		title, _ := cmd.Flags().GetString("title")
		content, _ := cmd.Flags().GetString("content")
		contentType, _ := cmd.Flags().GetString("content-type")
		tagsStr, _ := cmd.Flags().GetString("tags")

		if journalIDStr == "" {
			return fmt.Errorf("journal-id is required")
		}
		journalID, err := uuid.Parse(journalIDStr)
		if err != nil {
			return fmt.Errorf("invalid journal-id format: %w", err)
		}

		if title == "" {
			return fmt.Errorf("title is required")
		}
		if content == "" {
			return fmt.Errorf("content is required")
		}

		var tagsList []string
		if tagsStr != "" {
			tagsList = strings.Split(tagsStr, ",")
			for i, tag := range tagsList {
				tagsList[i] = strings.TrimSpace(tag)
			}
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		entry, err := memories.CreateEntry(dbConn, journalID, title, content, contentType, tagsList)
		if err != nil {
			return fmt.Errorf("failed to create entry: %w", err)
		}

		fmt.Println("Entry created successfully:")
		output, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format entry output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var entryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List entries",
	Long:  `Lists entries, optionally filtered by journal ID and/or tags. If tags are provided, entries must have all specified tags.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		var journalID *uuid.UUID
		journalIDStr, _ := cmd.Flags().GetString("journal-id")
		if journalIDStr != "" {
			jID, err := uuid.Parse(journalIDStr)
			if err != nil {
				return fmt.Errorf("invalid journal-id format: %w", err)
			}
			journalID = &jID
		}

		tagsStr, _ := cmd.Flags().GetString("tags")
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

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		entries, err := memories.ListEntries(dbConn, journalID, tagsList)
		if err != nil {
			return fmt.Errorf("failed to list entries: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No entries found matching the criteria.")
			return nil
		}

		fmt.Println("Entries:")
		output, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format entries output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var entryGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get a specific entry by its ID",
	Long:  `Retrieves and displays the details of a specific entry, including its tags, using its UUID.`,
	Args:  cobra.ExactArgs(1), // Requires exactly one argument: the ID
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID format: %w", err)
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		entry, err := memories.GetEntryByID(dbConn, entryID)
		if err != nil {
			return fmt.Errorf("failed to get entry: %w", err)
		}

		if entry == nil {
			fmt.Printf("Entry with ID %s not found.\n", entryIDStr)
			return nil
		}

		fmt.Printf("Entry (ID: %s):\n", entryIDStr)
		output, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format entry output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var entryUpdateCmd = &cobra.Command{
	Use:   "update [id]",
	Short: "Update an existing entry's title, content, or content_type",
	Long:  `Updates an existing entry with the given ID. Only provided fields (title, content, content-type) will be updated. Tags are managed separately via 'entries tag'.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID format: %w", err)
		}

		var title, content, contentType *string

		if cmd.Flags().Changed("title") {
			t, _ := cmd.Flags().GetString("title")
			title = &t
		}
		if cmd.Flags().Changed("content") {
			c, _ := cmd.Flags().GetString("content")
			content = &c
		}
		if cmd.Flags().Changed("content-type") {
			ct, _ := cmd.Flags().GetString("content-type")
			contentType = &ct
		}

		if title == nil && content == nil && contentType == nil {
			fmt.Println("No update fields provided. Use --title, --content, or --content-type.")
			// Optionally, one could fetch and display the current entry here.
			return nil
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		updatedEntry, err := memories.UpdateEntry(dbConn, entryID, title, content, contentType)
		if err != nil {
			return fmt.Errorf("failed to update entry: %w", err)
		}

		if updatedEntry == nil {
			// This handles the case where UpdateEntry returned nil, nil (e.g., entry not found)
			fmt.Printf("Entry with ID %s not found or no update performed (if no specific fields were changed).\n", entryIDStr)
			return nil
		}

		fmt.Println("Entry updated successfully:")
		output, err := json.MarshalIndent(updatedEntry, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format entry output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

var entryDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete an entry by its ID",
	Long:  `Deletes a specific entry from the database using its UUID. Associated tags will also be unlinked.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID format: %w", err)
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		err = memories.DeleteEntry(dbConn, entryID)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("Entry with ID %s not found.\n", entryIDStr)
				return nil // Not an error for the CLI if not found
			}
			return fmt.Errorf("failed to delete entry: %w", err)
		}

		fmt.Printf("Entry with ID %s deleted successfully.\n", entryIDStr)
		return nil
	},
}

var entryTagCmd = &cobra.Command{
	Use:   "tag [id]",
	Short: "Add or remove tags for an entry",
	Long:  `Manages tags for a specific entry by adding new tags and/or removing existing ones.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		entryIDStr := args[0]
		entryID, err := uuid.Parse(entryIDStr)
		if err != nil {
			return fmt.Errorf("invalid entry ID format: %w", err)
		}

		addTagsStr, _ := cmd.Flags().GetString("add")
		removeTagsStr, _ := cmd.Flags().GetString("remove")

		if addTagsStr == "" && removeTagsStr == "" {
			fmt.Println("No tags provided to add or remove. Use --add and/or --remove flags.")
			return nil
		}

		var tagsToAdd []string
		if addTagsStr != "" {
			tsl := strings.Split(addTagsStr, ",")
			for _, tag := range tsl {
				t := strings.TrimSpace(tag)
				if t != "" {
					tagsToAdd = append(tagsToAdd, t)
				}
			}
		}

		var tagsToRemove []string
		if removeTagsStr != "" {
			tsl := strings.Split(removeTagsStr, ",")
			for _, tag := range tsl {
				t := strings.TrimSpace(tag)
				if t != "" {
					tagsToRemove = append(tagsToRemove, t)
				}
			}
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		err = memories.ManageEntryTags(dbConn, entryID, tagsToAdd, tagsToRemove)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("Entry with ID %s not found.\n", entryIDStr)
				return nil // Not a CLI error if entry not found for tagging
			}
			return fmt.Errorf("failed to manage tags for entry: %w", err)
		}

		fmt.Println("Tags managed successfully. Current entry details:")
		// Fetch and print the updated entry
		updatedEntry, err := memories.GetEntryByID(dbConn, entryID)
		if err != nil {
			// This might happen if the entry was somehow deleted by another process concurrently,
			// or if GetEntryByID has an issue. Should be rare if ManageEntryTags succeeded.
			return fmt.Errorf("failed to fetch updated entry details: %w", err)
		}
		if updatedEntry == nil { // Should not happen if ManageEntryTags didn't return ErrNoRows
			fmt.Printf("Entry with ID %s not found after attempting to manage tags.\n", entryIDStr)
			return nil
		}

		output, err := json.MarshalIndent(updatedEntry, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format entry output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

// New command for tags
var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "Manage tags",
	Long:  `Provides commands for listing tags.`,
}

var tagListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all unique tags",
	Long:  `Lists all unique tags currently stored in the database, along with their creation and last update times.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL") // Assuming WAL and NORMAL sync
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		tags, err := memories.ListTags(dbConn)
		if err != nil {
			return fmt.Errorf("failed to list tags: %w", err)
		}

		if len(tags) == 0 {
			fmt.Println("No tags found.")
			return nil
		}

		fmt.Println("Tags:")
		output, err := json.MarshalIndent(tags, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format tags output: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

// New command for search
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for entries based on criteria (currently only by tags)",
	Long:  `Searches for entries. Currently, this command only supports searching by tags. Entries matching all provided tags will be returned.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbPath == "" {
			return fmt.Errorf("database path must be set using the --dbpath flag")
		}

		tagsStr, _ := cmd.Flags().GetString("tags")
		if tagsStr == "" {
			// Since --tags is marked as required, Cobra should enforce this.
			// This is an additional safeguard or for if MarkFlagRequired fails/is removed.
			return fmt.Errorf("the --tags flag is required for search")
		}

		var tagsList []string
		tsl := strings.Split(tagsStr, ",")
		for _, tag := range tsl {
			t := strings.TrimSpace(tag)
			if t != "" {
				tagsList = append(tagsList, t)
			}
		}
		if len(tagsList) == 0 {
			return fmt.Errorf("no valid tags provided in the --tags flag")
		}

		dbConn, err := pkgdb.OpenDBConnection(dbPath, true, "NORMAL")
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer dbConn.Close()

		// Use ListEntries with nil journalID to search across all journals by tags
		entries, err := memories.ListEntries(dbConn, nil, tagsList)
		if err != nil {
			return fmt.Errorf("failed to search entries by tags: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No entries found matching the specified tags.")
			return nil
		}

		fmt.Println("Search results (Entries matching all tags):")
		output, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format search results: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}

func initCmd() {
	rootCmd.PersistentFlags().StringVar(&dbPath, "dbpath", "", "Path to the Mnemonic SQLite database file (e.g., ./mnemonic.db)")

	dbUpgradeCmd.Flags().Bool("wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode.")
	dbUpgradeCmd.Flags().String("sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA).")
	dbCmd.AddCommand(dbUpgradeCmd)

	journalCreateCmd.Flags().StringP("name", "n", "", "Name of the journal (required)")
	journalCreateCmd.MarkFlagRequired("name")
	journalCreateCmd.Flags().StringP("description", "d", "", "Description of the journal")

	journalUpdateCmd.Flags().StringP("name", "n", "", "New name for the journal")
	journalUpdateCmd.Flags().StringP("description", "d", "", "New description for the journal")
	journalUpdateCmd.Flags().Bool("active", false, "Set journal to active")
	journalUpdateCmd.Flags().Bool("inactive", false, "Set journal to inactive")

	journalsCmd.AddCommand(journalCreateCmd, journalListCmd, journalGetCmd, journalUpdateCmd, journalDeleteCmd)

	// Add flags for entry create
	entryCreateCmd.Flags().StringP("journal-id", "j", "", "ID of the journal to add the entry to (required)")
	entryCreateCmd.MarkFlagRequired("journal-id")
	entryCreateCmd.Flags().StringP("title", "t", "", "Title of the entry (required)")
	entryCreateCmd.MarkFlagRequired("title")
	entryCreateCmd.Flags().StringP("content", "c", "", "Content of the entry (required)")
	entryCreateCmd.MarkFlagRequired("content")
	entryCreateCmd.Flags().String("content-type", "text/plain", "Content type of the entry (e.g., text/markdown)")
	entryCreateCmd.Flags().String("tags", "", "Comma-separated list of tags for the entry")

	// Add flags for entry list
	entryListCmd.Flags().StringP("journal-id", "j", "", "ID of the journal to list entries from (optional)")
	entryListCmd.Flags().StringP("tags", "t", "", "Comma-separated list of tags to filter entries by (optional, entry must have all specified tags)")

	// Flags for entry update
	entryUpdateCmd.Flags().StringP("title", "t", "", "New title for the entry")
	entryUpdateCmd.Flags().StringP("content", "c", "", "New content for the entry")
	entryUpdateCmd.Flags().String("content-type", "", "New content type for the entry (e.g., text/markdown)")

	// Flags for entry tag management
	entryTagCmd.Flags().String("add", "", "Comma-separated list of tags to add to the entry")
	entryTagCmd.Flags().String("remove", "", "Comma-separated list of tags to remove from the entry")

	entriesCmd.AddCommand(entryCreateCmd, entryListCmd, entryGetCmd, entryUpdateCmd, entryDeleteCmd, entryTagCmd)

	// Add tags command and its subcommands
	tagsCmd.AddCommand(tagListCmd)

	// Add flags for search command
	searchCmd.Flags().StringP("tags", "t", "", "Comma-separated list of tags to search for (required, entry must have all specified tags)")
	searchCmd.MarkFlagRequired("tags")

	rootCmd.AddCommand(completionCmd, versionCmd, dbCmd, journalsCmd, entriesCmd, tagsCmd, searchCmd, serverCmd)
}

func main() {
	initCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
