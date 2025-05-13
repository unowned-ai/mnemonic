//go:build tui

package main

import (
	"github.com/unowned-ai/recall/pkg/tui"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Show terminal UI",
	Long:  `Display an interactive terminal UI for browsing data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dbConn, err := openDB()
		if err != nil {
			return err
		}
		defer dbConn.Close()

		return tui.ShowTUI(dbConn)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
