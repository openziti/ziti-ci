package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

type updateGoDepCmd struct {
	baseCommand
}

func (cmd *updateGoDepCmd) execute() {
	cmd.runGitCommand("Allow fetching other branches", "config", "--replace-all", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	cmd.runGitCommand("Ensure origin/master is up to date", "fetch", "origin", "master")
	cmd.runGitCommand("Ensure go.mod/go.sum are untouched", "checkout", "--", "go.mod", "go.sum")

	if !isManualCompleteProject() {
		cmd.runGitCommand("Sync with master", "merge", "--ff-only", "origin/master")

		output := cmd.runCommandWithOutput("Ensure we are synced", "git", "diff", "origin/master")
		if len(output) != 0 {
			cmd.failf("update branch has diverged from master. automated merges won't work until this is fixed. Diff: %+v", strings.Join(output, "\n"))
		}
	}

	dep := cmd.getUpdatedDep()
	cmd.runCommand("Update dependency", "go", "get", dep)
	diffOutput := cmd.runCommandWithOutput("check if there's a change", "git", "diff", "--name-only", "go.mod")
	if len(diffOutput) != 1 || diffOutput[0] != "go.mod" {
		_, _ = fmt.Fprintf(cmd.cmd.ErrOrStderr(), "requested dependency did not result in change\n")
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(cmd.cmd.OutOrStdout(), "attempting to update to %v\n", dep)

	cmd.runCommand("Tidy go.sum", "go", "mod", "tidy")
	cmd.runGitCommand("Add go mod changes", "add", "go.mod", "go.sum")
	cmd.runGitCommand("Commit go.mod changes", "commit", "-m", fmt.Sprintf("Updating dependency %v", dep))
}

func (cmd *updateGoDepCmd) getUpdatedDep() string {
	newDep := ""
	if len(cmd.args) > 0 {
		newDep = cmd.args[0]
	}
	if newDep == "" {
		newDep = os.Getenv("UPDATED_DEPENDENCY")
	}

	if newDep == "" {
		cmd.failf("no updated dependency provided\n")
	}

	return newDep
}

func newUpdateGoDepCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "update-go-dependency",
		Short: "Update a go dependency to a different version",
		Args:  cobra.MaximumNArgs(1),
	}

	result := &updateGoDepCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	return finalize(result)
}

type completeUpdateGoDepCmd struct {
	baseCommand
}

func (cmd *completeUpdateGoDepCmd) execute() {
	// go get gox or go get jfrog can mess with go.mod since we committed
	cmd.runGitCommand("Ensure go.mod/go.sum are untouched", "checkout", "--", "go.mod", "go.sum")
	currentCommit := cmd.getCmdOutputOneLine("get git SHA", "git", "rev-parse", "--short=12", "HEAD")
	if !isManualCompleteProject() {
		cmd.runGitCommand("Checkout master", "checkout", "master")
	} else {
		cmd.runGitCommand("Checkout actual branch", "checkout", cmd.getCurrentBranch())
	}
	cmd.runGitCommand("Merge in changes", "merge", "--ff-only", currentCommit)
	cmd.runGitCommand("Push to remote", "push")
}

func newCompleteUpdateGoDepCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "complete-update-go-dependency",
		Short: "Merge a go dependency update to master and push",
		Args:  cobra.ExactArgs(0),
	}

	result := &completeUpdateGoDepCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	return finalize(result)
}

func isManualCompleteProject() bool {
	return "true" == os.Getenv("complete_update_dependency_manually")
}
