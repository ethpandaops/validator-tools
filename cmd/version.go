package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	Release   = "dev"
	GitCommit = "none"
	GOOS      = runtime.GOOS
	GOARCH    = runtime.GOARCH
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of validator tools.",
	Long:  `Prints the version of validator tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		initCommon()

		fmt.Printf("Version: %s\nCommit: %s\nOS/Arch: %s/%s\n",
			Release, GitCommit, GOOS, GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
