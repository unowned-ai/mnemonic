package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/unowned-ai/mnemonic/pkg"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "mnemonic",
	Short:   "",
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

func initCmd() {
	rootCmd.AddCommand(completionCmd, versionCmd)
}

func main() {
	initCmd()

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
