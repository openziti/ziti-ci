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
	"os"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

type buildSdkReleaseNotesCmd struct {
	baseBuildReleaseNotesCmd
}

func (cmd *buildSdkReleaseNotesCmd) Execute() {
	cmd.initVersions()
	cmd.generateSdkReleaseNotes()
}

// initVersions sets quiet mode, handles an optional StartVersion, and evaluates
// the current and next version from tags.
func (cmd *buildSdkReleaseNotesCmd) initVersions() {
	if !cmd.RootCobraCmd.Flags().Changed("quiet") {
		cmd.quiet = true
	}

	if cmd.StartVersion != "" {
		v, err := version.NewVersion(cmd.StartVersion)
		if err != nil {
			panic(err)
		}
		cmd.CurrentVersion = v
		cmd.EvalCurrentAndNextVersion()
		return
	}

	cmd.EvalCurrentAndNextVersion()
	cmd.evalStartVersion()
}

// generateSdkReleaseNotes outputs the full release notes section for the SDK,
// including the version header and dependency updates.
func (cmd *buildSdkReleaseNotesCmd) generateSdkReleaseNotes() {
	cmd.printf("# Release notes %v\n", cmd.NextVersion)
	cmd.printf("\n## Issues Fixed and Dependency Updates\n")
	cmd.printf("\n")
	cmd.generateSdkDependencyUpdates()
}

// generateSdkDependencyUpdates outputs the dependency update bullet points by
// diffing go.mod against the previous version.
func (cmd *buildSdkReleaseNotesCmd) generateSdkDependencyUpdates() {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		panic(err)
	}

	newGoMod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		panic(err)
	}

	output := cmd.runCommandWithOutput("get go.mod contents", "git", "show", fmt.Sprintf("v%v:go.mod", cmd.CurrentVersion))
	data = []byte(strings.Join(output, "\n"))
	oldGoMod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		panic(err)
	}

	cmd.printf("* %v: [v%v -> v%v](https://github.com/openziti/sdk-golang/compare/v%v...v%v)\n",
		newGoMod.Module.Mod.Path, cmd.CurrentVersion, cmd.NextVersion, cmd.CurrentVersion, cmd.NextVersion)
	if err = cmd.GetChanges("sdk-golang", "v"+cmd.CurrentVersion.String(), "HEAD"); err != nil {
		panic(err)
	}

	oldVersions := map[string]*modfile.Require{}

	for _, m := range oldGoMod.Require {
		oldVersions[m.Mod.Path] = m
	}

	for _, m := range newGoMod.Require {
		prev, found := oldVersions[m.Mod.Path]
		if !found {
			path := m.Mod.Path
			prevVersion := cmd.getPreviousVersion(path)
			for prevVersion != nil {
				prev, found = oldVersions[*prevVersion]
				if found {
					break
				}
				prevVersion = cmd.getPreviousVersion(*prevVersion)
			}
		}
		if !found {
			cmd.printf("* %v: %v (new)\n", m.Mod.Path, m.Mod.Version)
		} else if m.Mod.Version != prev.Mod.Version {
			if strings.Contains(m.Mod.Path, "openziti") {
				project := strings.Split(m.Mod.Path, "/")[2]
				cmd.printf("* %v: [%v -> %v](https://github.com/openziti/%v/compare/%v...%v)\n",
					m.Mod.Path, prev.Mod.Version, m.Mod.Version, project, prev.Mod.Version, m.Mod.Version)
				if err = cmd.GetChanges(project, prev.Mod.Version, m.Mod.Version); err != nil {
					panic(err)
				}
			} else {
				cmd.printf("* %v: %v -> %v\n", m.Mod.Path, prev.Mod.Version, m.Mod.Version)
			}

		} else if cmd.ShowUnchanged {
			cmd.printf("* %v: %v (unchanged)\n", m.Mod.Path, m.Mod.Version)
		}
	}
}

func newBuildSdkReleaseNotesCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "build-sdk-release-notes",
		Short: "Prints out the release notes for the latest or a given version",
		Args:  cobra.MaximumNArgs(1),
	}

	result := &buildSdkReleaseNotesCmd{
		baseBuildReleaseNotesCmd: baseBuildReleaseNotesCmd{
			BaseCommand: BaseCommand{
				RootCommand: root,
				Cmd:         cobraCmd,
			},
		},
	}

	cobraCmd.Flags().BoolVarP(&result.AllCommits, "all-commits", "a", false, "Show all commits, not just closed issues")
	cobraCmd.Flags().BoolVarP(&result.ShowUnchanged, "show-unchanged", "u", false, "Show OpenZiti upstream libraries, even if unchanged")
	cobraCmd.Flags().BoolVar(&result.NoPrScan, "no-pr-scan", false, "Don't inspect pull requests for issue links missing from commit messages")

	return Finalize(result)
}
