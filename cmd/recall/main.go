package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	recall "github.com/unowned-ai/recall/pkg"
	pkgdb "github.com/unowned-ai/recall/pkg/db"

	"github.com/spf13/cobra"
)

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

		if dbPath == "" {
			return errors.New("database path is required")
		}

		fmt.Printf("Attempting to upgrade memoriesdb component in database at: %s (WAL: %t, Sync: %s)\n", dbPath, walMode, syncMode)

		dbConn, err := pkgdb.OpenDBConnection(dbPath, walMode, syncMode)
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

func initCmd() {
	// Define persistent DB flags on rootCmd so all commands can use them
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (optional for mcp command, uses system-specific default if not provided)")
	rootCmd.PersistentFlags().BoolVar(&walMode, "wal", false, "Enable SQLite WAL (Write-Ahead Logging) mode (default: false)")
	rootCmd.PersistentFlags().StringVar(&syncMode, "sync", "FULL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA) (default: FULL)")
	// It's often better to mark required flags on the specific commands that need them,
	// or use PersistentPreRunE on rootCmd to validate if dbPath is always needed.
	// For now, individual commands like dbUpgrade, entries, journals, tags, search
	// will rely on openDB() checking dbPath or their own MarkFlagRequired if they have it.
	// Or, if "db" is truly global, rootCmd.MarkPersistentFlagRequired("db") could be used.

	dbUpgradeCmd.MarkFlagRequired("db")

	dbCmd.AddCommand(dbUpgradeCmd)

	initJournalsCmd()
	initEntriesCmd()
	initTagsCmd()
	initSearchCmd()
	rootCmd.AddCommand(completionCmd, versionCmd, dbCmd, journalsCmd, entriesCmd, tagsCmd, searchCmd, mcpCmd)
}

func main() {
	initCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
