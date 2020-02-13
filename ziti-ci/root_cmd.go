package main

import (
	"github.com/spf13/cobra"
)

type langType int

const (
	LangGo langType = 1
)

type rootCommand struct {
	rootCobraCmd *cobra.Command

	verbose bool
	dryRun  bool

	langName string
	lang     langType

	baseVersionString string
	baseVersionFile   string
}

func newRootCommand() *rootCommand {
	cobraCmd := &cobra.Command{
		Use:   "ziti-ci",
		Short: "Ziti CI Tool",
	}

	var rootCmd = &rootCommand{
		rootCobraCmd: cobraCmd,
	}

	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.verbose, "verbose", "v", false, "enable verbose output")
	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.dryRun, "dry-run", "d", false, "do a dry run")
	cobraCmd.PersistentFlags().StringVarP(&rootCmd.langName, "language", "l", "go", "enable language specific settings. Valid values: [go]")

	cobraCmd.PersistentFlags().StringVarP(&rootCmd.baseVersionString, "base-version", "b", "", "set base version")
	cobraCmd.PersistentFlags().StringVarP(&rootCmd.baseVersionFile, "base-version-file", "f", DefaultVersionFile, "set base version file location")

	return rootCmd
}
