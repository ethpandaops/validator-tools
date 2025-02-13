package cmd

import (
	"github.com/spf13/cobra"
)

var depositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Deposit tools",
	Long:  `Deposit tools.`,
}

func init() {
	rootCmd.AddCommand(depositCmd)
}
