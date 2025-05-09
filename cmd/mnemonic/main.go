package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	mnemonic "github.com/unowned-ai/mnemonic/pkg"
	pkgdb "github.com/unowned-ai/mnemonic/pkg/db"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "mnemonic",
	Short:   "A self-hostable datastore for your memories to share with your AI models.",
	Long:    ``,
	Version: fmt.Sprintf("v%s", mnemonic.Version),
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
	Long: `Connects to the SQLite database at the specified path (provided with the --db flag) and applies any necessary
schema migrations to bring the memoriesdb component up to the current application schema version.
If the database does not exist or is uninitialized for this component, it will be created
and initialized with the latest schema for the memoriesdb component.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		walEnabled, _ := cmd.Flags().GetBool("wal")
		syncMode, _ := cmd.Flags().GetString("sync")

		if dbPath == "" {
			return errors.New("database path is required")
		}

		fmt.Printf("Attempting to upgrade memoriesdb component in database at: %s (WAL: %t, Sync: %s)\n", dbPath, walEnabled, syncMode)

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

func initCmd() {
	// Define persistent DB flags on rootCmd so all commands can use them
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to the database file (required)")
	rootCmd.PersistentFlags().BoolVar(&walMode, "wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode")
	rootCmd.PersistentFlags().StringVar(&syncMode, "sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA)")
	// It's often better to mark required flags on the specific commands that need them,
	// or use PersistentPreRunE on rootCmd to validate if dbPath is always needed.
	// For now, individual commands like dbUpgrade, entries, journals, tags, search
	// will rely on openDB() checking dbPath or their own MarkFlagRequired if they have it.
	// Or, if "db" is truly global, rootCmd.MarkPersistentFlagRequired("db") could be used.

	dbUpgradeCmd.Flags().StringVar(&dbPath, "db", "", "Path to the database file (required)")
	dbUpgradeCmd.Flags().Bool("wal", true, "Enable SQLite WAL (Write-Ahead Logging) mode.")
	dbUpgradeCmd.Flags().String("sync", "NORMAL", "SQLite synchronous pragma (OFF, NORMAL, FULL, EXTRA).")
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
