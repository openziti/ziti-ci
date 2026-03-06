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
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const componentUpdatesHeader = "## Component Updates and Bug Fixes"

type updateReleaseNotesCmd struct {
	buildReleaseNotesCmd
	ChangelogFile string
}

func (cmd *updateReleaseNotesCmd) Execute() {
	var buf bytes.Buffer
	cmd.Writer = &buf

	cmd.initVersions()
	cmd.generateReleaseNotes()

	newSection := buf.String()

	data, err := os.ReadFile(cmd.ChangelogFile)
	if err != nil {
		panic(fmt.Errorf("unable to read changelog file %v: %w", cmd.ChangelogFile, err))
	}

	content := string(data)

	sectionIdx := strings.Index(content, componentUpdatesHeader)
	if sectionIdx == -1 {
		panic(fmt.Errorf("unable to find %q section in %v", componentUpdatesHeader, cmd.ChangelogFile))
	}

	// Find the end of this section: the next heading at the same or higher level (## or #), or EOF
	afterHeader := sectionIdx + len(componentUpdatesHeader)
	endIdx := len(content)
	remaining := content[afterHeader:]
	searchIdx := 0
	for searchIdx < len(remaining) {
		nlIdx := strings.Index(remaining[searchIdx:], "\n")
		if nlIdx == -1 {
			break
		}
		lineStart := searchIdx + nlIdx + 1
		if lineStart < len(remaining) && remaining[lineStart] == '#' {
			endIdx = afterHeader + lineStart
			break
		}
		searchIdx = lineStart
	}

	var result strings.Builder
	result.WriteString(content[:sectionIdx])
	result.WriteString(componentUpdatesHeader)
	result.WriteString("\n\n")
	result.WriteString(newSection)
	result.WriteString("\n")
	if endIdx < len(content) {
		result.WriteString(content[endIdx:])
	}

	if err := os.WriteFile(cmd.ChangelogFile, []byte(result.String()), 0644); err != nil {
		panic(fmt.Errorf("unable to write changelog file %v: %w", cmd.ChangelogFile, err))
	}

	fmt.Printf("Updated %q section in %v\n", componentUpdatesHeader, cmd.ChangelogFile)
}

func newUpdateReleaseNotesCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "update-release-notes",
		Short: "Builds release notes and updates the Component Updates and Bug Fixes section in the changelog",
		Args:  cobra.MaximumNArgs(1),
	}

	result := &updateReleaseNotesCmd{
		buildReleaseNotesCmd: buildReleaseNotesCmd{
			baseBuildReleaseNotesCmd: baseBuildReleaseNotesCmd{
				BaseCommand: BaseCommand{
					RootCommand: root,
					Cmd:         cobraCmd,
				},
			},
		},
		ChangelogFile: "CHANGELOG.md",
	}

	cobraCmd.Flags().BoolVarP(&result.AllCommits, "all-commits", "a", false, "Show all commits, not just closed issues")
	cobraCmd.Flags().BoolVarP(&result.ShowUnchanged, "show-unchanged", "u", false, "Show OpenZiti upstream libraries, even if unchanged")
	cobraCmd.Flags().StringVarP(&result.StartVersion, "start-version", "s", "", "Version to use a starting point when diffing against")
	cobraCmd.Flags().StringVarP(&result.ChangelogFile, "changelog-file", "c", "CHANGELOG.md", "Path to the changelog file to update")

	return Finalize(result)
}
