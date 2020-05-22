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

const (
	DefaultVersionFile = "./version"
)

type tagCmd struct {
	BaseCommand
	onlyForBranch string
}

func (cmd *tagCmd) Execute() {
	if cmd.onlyForBranch != "" && cmd.onlyForBranch != cmd.GetCurrentBranch() {
		cmd.Infof("current branch %v doesn't match requested branch %v, so skipping\n", cmd.GetCurrentBranch(), cmd.onlyForBranch)
		os.Exit(0)
	}
	cmd.EvalCurrentAndNextVersion()

	headTags := cmd.getVersionList("tag", "--points-at", "HEAD")
	if len(headTags) > 0 {
		cmd.Errorf("head already tagged with %+v:\n", headTags)
		os.Exit(0)
	}

	cmd.Infof("previous version: %v, next version: %v\n", cmd.CurrentVersion, cmd.NextVersion)

	if cmd.isGoLang() {
		nextMajorVersion := cmd.NextVersion.Segments()[0]
		if nextMajorVersion > 1 {
			moduleName := cmd.getModule()
			if !strings.HasSuffix(moduleName, fmt.Sprintf("/v%v", nextMajorVersion)) {
				cmd.Failf("error: module version doesn't match next version: %v\n", nextMajorVersion)
			}
		}
	}

	tagVersion := fmt.Sprintf("%v", cmd.NextVersion)
	if cmd.isGoLang() {
		tagVersion = "v" + tagVersion
	}
	tagParms := []string{"tag", "-a", tagVersion, "-m", fmt.Sprintf("Release %v", tagVersion)}
	cmd.RunGitCommand("create tag", tagParms...)
	cmd.RunGitCommand("push tag to repo", "push", "origin", tagVersion)
}

func newTagCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "tag",
		Short: "Tag and push command",
		Args:  cobra.ExactArgs(0),
	}

	result := &tagCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.PersistentFlags().StringVar(&result.onlyForBranch, "only-for-branch", "", "Only do if branch matches")

	return Finalize(result)
}
