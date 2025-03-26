package cmd

import (
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify various validator data",
	Long:  `Verify various validator data including deposit data and voluntary exits.`,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}
