/*
 * Copyright NetFoundry, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

type updateGoDepCmd struct {
	BaseCommand
}

func (cmd *updateGoDepCmd) Execute() {
	cmd.RunGitCommand("Allow fetching other branches", "config", "--replace-all", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	//seems to have broken update deps... cmd.RunGitCommand("Ensure " + cmd.GetCurrentBranch() + " is up to date", "fetch", "origin", cmd.GetCurrentBranch())
	cmd.RunGitCommand("Ensure origin/main is up to date", "fetch", "origin", "main")
	cmd.RunGitCommand("Ensure go.mod/go.sum are untouched", "checkout", "--", "go.mod", "go.sum")

	if !isManualCompleteProject() {
		cmd.RunGitCommand("Sync with main", "merge", "--ff-only", "origin/main")

		output := cmd.runCommandWithOutput("Ensure we are synced", "git", "diff", "origin/main")
		if len(output) != 0 {
			cmd.Failf("update branch has diverged from main. automated merges won't work until this is fixed. Diff: %+v", strings.Join(output, "\n"))
		}
	}

	dep := cmd.getUpdatedDep()
	cmd.runCommand("Update dependency", "go", "get", dep)
	diffOutput := cmd.runCommandWithOutput("check if there's a change", "git", "diff", "--name-only", "go.mod")
	if len(diffOutput) != 1 || diffOutput[0] != "go.mod" {
		_, _ = fmt.Fprintf(cmd.Cmd.ErrOrStderr(), "requested dependency did not result in change\n")
		os.Exit(0)
	}
	_, _ = fmt.Fprintf(cmd.Cmd.OutOrStdout(), "attempting to update to %v\n", dep)

	cmd.runCommand("Tidy go.sum targetting 1.16", "go", "mod", "tidy", "-go=1.16")
	cmd.runCommand("Tidy go.sum targetting 1.17", "go", "mod", "tidy", "-go=1.17")
	cmd.RunGitCommand("Add go mod changes", "add", "go.mod", "go.sum")
	cmd.RunGitCommand("Commit go.mod changes", "commit", "-m", fmt.Sprintf("Updating dependency %v", dep))
}

func (cmd *updateGoDepCmd) getUpdatedDep() string {
	newDep := ""
	if len(cmd.Args) > 0 {
		newDep = cmd.Args[0]
	}
	if newDep == "" {
		newDep = os.Getenv("UPDATED_DEPENDENCY")
	}

	if newDep == "" {
		cmd.Failf("no updated dependency provided\n")
	}

	return newDep
}

func newUpdateGoDepCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "update-go-dependency <updated-dependency>",
		Short: "Update a go dependency to a different version",
		Args:  cobra.MaximumNArgs(1),
	}

	result := &updateGoDepCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return Finalize(result)
}

type completeUpdateGoDepCmd struct {
	BaseCommand
}

func (cmd *completeUpdateGoDepCmd) Execute() {
	updateBranch := cmd.GetCurrentBranch()

	// go get gox or go get jfrog can mess with go.mod since we committed
	cmd.RunGitCommand("Ensure go.mod/go.sum are untouched", "checkout", "--", "go.mod", "go.sum")
	currentCommit := cmd.GetCmdOutputOneLine("get git SHA", "git", "rev-parse", "--short=12", "HEAD")
	if !isManualCompleteProject() {
		cmd.RunGitCommand("Checkout main", "checkout", "main")
	} else {
		cmd.RunGitCommand("Checkout actual branch", "checkout", cmd.GetCurrentBranch())
	}
	cmd.RunGitCommand("Merge in changes", "merge", "--ff-only", currentCommit)
	cmd.RunGitCommand("Push to remote", "push")
	cmd.RunGitCommand("Push update branch ", "push", "origin", updateBranch)
}

func newCompleteUpdateGoDepCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "complete-update-go-dependency",
		Short: "Merge a go dependency update to main and push",
		Args:  cobra.ExactArgs(0),
	}

	result := &completeUpdateGoDepCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return Finalize(result)
}

func isManualCompleteProject() bool {
	return "true" == os.Getenv("complete_update_dependency_manually")
}
