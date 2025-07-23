package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

//nolint:gochecknoglobals // Build-time variables set via ldflags
var (
	// Release is the current version of the validator tools.
	Release = "dev"
	// GitCommit is the git commit hash of the build.
	GitCommit = "none"
	// GOOS is the target operating system.
	GOOS = runtime.GOOS
	// GOARCH is the target architecture.
	GOARCH = runtime.GOARCH
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Prints the version of validator tools.",
	Long:  `Prints the version of validator tools.`,
	Run: func(_ *cobra.Command, _ []string) {
		initCommon()

		fmt.Printf("Version: %s\nCommit: %s\nOS/Arch: %s/%s\n",
			Release, GitCommit, GOOS, GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
