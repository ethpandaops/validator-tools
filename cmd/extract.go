package cmd

import (
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract various validator data",
	Long:  `Extract various validator data including deposit data and voluntary exits.`,
}

func init() {
	rootCmd.AddCommand(extractCmd)
}
