package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func main() {
	rootCmd := newRootCommand()
	rootCobraCmd := rootCmd.rootCobraCmd

	rootCobraCmd.AddCommand(newTagCmd(rootCmd))
	rootCobraCmd.AddCommand(newBuildInfoCmd(rootCmd))
	rootCobraCmd.AddCommand(newConfigureGitCmd(rootCmd))
	rootCobraCmd.AddCommand(newUpdateGoDepCmd(rootCmd))
	rootCobraCmd.AddCommand(newCompleteUpdateGoDepCmd(rootCmd))
	rootCobraCmd.AddCommand(newTriggerTravisBuildCmd(rootCmd))

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show build information",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ziti-cmd version: %v, revision: %v, branch: %v, build-by: %v, built-on: %v\n",
				Version, Revision, Branch, BuildUser, BuildDate)
		},
	}

	rootCobraCmd.AddCommand(versionCmd)

	if err := rootCmd.rootCobraCmd.Execute(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(-1)
	}
}
