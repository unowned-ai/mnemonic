package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	recall "github.com/unowned-ai/recall/pkg"
	pkgdb "github.com/unowned-ai/recall/pkg/db"

	"github.com/spf13/cobra"
)

// Global flags
var dbPath string
var walMode bool
var syncMode string

var rootCmd = &cobra.Command{
	Use:     "recall",
	Short:   "A self-hostable datastore for your memories to share with your AI models.",
	Long:    ``,
	Version: fmt.Sprintf("v%s", recall.Version),
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// This is a good place to ensure dbPath is usable if needed globally before RunE
		// For now, openDB handles the check.
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var completionShells = []string{"bash", "zsh", "fish", "powershell"}

var completionCmd = &cobra.Command{
	Use:   fmt.Sprintf("completion %s", strings.Join(completionShells, "|")),
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for recall.

The command prints a completion script to stdout. You can source it in your shell
or install it to the appropriate location for your shell to enable completions permanently.

Examples:

  Bash (current shell):
    $ source <(recall completion bash)

  Bash (persist):
    $ recall completion bash > /etc/bash_completion.d/recall

  Zsh:
    $ recall completion zsh > "${fpath[1]}/_recall"

  Fish:
    $ recall completion fish | source
    $ recall completion fish > ~/.config/fish/completions/recall.fish

  PowerShell:
    PS> recall completion powershell | Out-String | Invoke-Expression`,
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
	Short: "Print the version number of recall",
	Long:  `All software has versions. This is recall's`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(recall.Version)
	},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Manage the recall database",
	Long:  `Provides commands for managing the Recall SQLite database, including schema upgrades. GIGO.`,
}

var dbUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the Recall database schema to the latest version for the memoriesdb component",
	Long: `Connects to the SQLite database at the specified path (provided with the --db flag) and applies any necessary
schema migrations to bring the memoriesdb component up to the current application schema version.
If the database does not exist or is uninitialized for this component, it will be created
and initialized with the latest schema for the memoriesdb component.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// These flags are defined locally for dbUpgradeCmd but also persistently on rootCmd.
		// Cobra will use the most local definition when parsing.
		// For this command, dbPath is marked required locally.
		localDbPath, _ := cmd.Flags().GetString("db")
		localWalEnabled, _ := cmd.Flags().GetBool("wal")
		localSyncMode, _ := cmd.Flags().GetString("sync")

		if localDbPath == "" {
			// This should be caught by MarkFlagRequired, but as a safeguard:
			return errors.New("database path is required for db upgrade")
		}

		fmt.Printf("Attempting to upgrade memoriesdb component in database at: %s (WAL: %t, Sync: %s)\n", localDbPath, localWalEnabled, localSyncMode)

		dbConn, err := pkgdb.OpenDBConnection(localDbPath, localWalEnabled, localSyncMode)
		if err != nil {
			return err
		}
		defer dbConn.Close()

		if err := pkgdb.UpgradeDB(dbConn, localDbPath, pkgdb.TargetSchemaVersion); err != nil {
			return err
		}
		fmt.Println("Database upgrade successful.")
		return nil
	},
}

func initCmd() {
	// Define persistent DB flags on rootCmd so all commands can use them
	// These will populate the global dbPath, walMode, syncMode variables
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (uses system-specific default if not provided)")
	rootCmd.PersistentFlags().BoolVar(&walMode, "wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode (default true)")
	rootCmd.PersistentFlags().StringVar(&syncMode, "sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA) (default NORMAL)")

	// dbUpgradeCmd flags (local to the command, but we can let them use the globals too if not set)
	// However, dbUpgradeCmd specifically marks "db" as required for itself.
	dbUpgradeCmd.Flags().String("db", "", "Path to the database file (required for db upgrade)")
	dbUpgradeCmd.Flags().Bool("wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode.")
	dbUpgradeCmd.Flags().String("sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA).")
	dbUpgradeCmd.MarkFlagRequired("db") // This applies to dbUpgradeCmd only

	dbCmd.AddCommand(dbUpgradeCmd)

	initJournalsCmd()
	initEntriesCmd()
	initTagsCmd()
	initSearchCmd()
	rootCmd.AddCommand(completionCmd, versionCmd, dbCmd, journalsCmd, entriesCmd, tagsCmd, searchCmd, mcpCmd)
}

func main() {
	initCmd() // Initializes commands and flags

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
