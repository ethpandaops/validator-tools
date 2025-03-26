package cmd

import (
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate various validator data",
	Long:  `Generate various validator data including voluntary exits.`,
}

func init() {
	rootCmd.AddCommand(generateCmd)
}
