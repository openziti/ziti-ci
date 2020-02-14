package main

import "github.com/spf13/cobra"

type packageCmd struct {
	baseCommand
}

func (cmd *packageCmd) execute() {

}

func newPackageCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "package <destination> <files>",
		Short: "Packages files for release",
		Args:  cobra.MinimumNArgs(2),
	}

	result := &packageCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	return finalize(result)
}
