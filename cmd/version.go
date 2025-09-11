package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// These variables will be set at build time using ldflags
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of aft-pipeline-tool",
	Long:  `Print the version number, git commit, and build date of aft-pipeline-tool`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("aft-pipeline-tool version %s\n", Version)
		fmt.Printf("Git commit: %s\n", GitCommit)
		fmt.Printf("Build date: %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
