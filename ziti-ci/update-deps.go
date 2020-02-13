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
	cmd.runGitCommand("Sync with master", "merge", "--ff-only", "origin/master")
	dep := cmd.getUpdatedDep()
	cmd.runCommand("Update dependency", "go", "get", dep)
	diffOutput := cmd.runCommandWithOutput("check if there's a change", "git", "diff", "--name-only", "go.mod")
	if len(diffOutput) != 1 || diffOutput[0] != "go.mod" {
		_, _ = fmt.Fprintf(cmd.cmd.ErrOrStderr(), "requested dependency did not result in change\n")
		os.Exit(-1)
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
		newDep = os.Getenv("TRAVIS_COMMIT_MESSAGE")
		parts := strings.SplitN(newDep, " ", 1)
		if len(parts) != 2 {
			cmd.failf("commit message %v not a valid dependency update request: %v\n", newDep)
		}
		if parts[0] != "ziti-ci:update-dependency" {
			cmd.failf("commit message %v not a valid dependency update request: %v\n", newDep)
		}
		newDep = parts[1]
	}

	if newDep == "" {
		cmd.failf("no updated dependency provided\n")
	}

	return newDep
}

func newUpdateGoDepCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "update-go-dependency",
		Short: "update-go-dependency",
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
	currentCommit := cmd.getCmdOutputOneLine("get git SHA", "git", "rev-parse", "--short=12", "HEAD")
	cmd.runGitCommand("Checkout master", "checkout", "master")
	cmd.runGitCommand("Merge in update branch", "merge", "--ff-only", currentCommit)
	cmd.runGitCommand("Push to remote", "push")
}

func newCompleteUpdateGoDepCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "complete-update-go-dependency",
		Short: "complete-update-go-dependency",
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
