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

const sdkDependencyHeader = "## Issues Fixed and Dependency Updates"

// updateSdkReleaseNotesCmd adds or updates SDK release notes in a changelog file.
type updateSdkReleaseNotesCmd struct {
	buildSdkReleaseNotesCmd
	ChangelogFile string
}

func (cmd *updateSdkReleaseNotesCmd) Execute() {
	cmd.initVersions()

	data, err := os.ReadFile(cmd.ChangelogFile)
	if err != nil {
		panic(fmt.Errorf("unable to read changelog file %v: %w", cmd.ChangelogFile, err))
	}

	content := string(data)
	versionHeader := fmt.Sprintf("# Release notes %v\n", cmd.NextVersion)

	versionIdx := strings.Index(content, versionHeader)
	if versionIdx == -1 {
		// Version section not found — prepend full release notes
		var buf bytes.Buffer
		cmd.Writer = &buf
		cmd.generateSdkReleaseNotes()

		var result strings.Builder
		result.WriteString(buf.String())
		result.WriteString("\n")
		result.WriteString(content)

		if err := os.WriteFile(cmd.ChangelogFile, []byte(result.String()), 0644); err != nil {
			panic(fmt.Errorf("unable to write changelog file %v: %w", cmd.ChangelogFile, err))
		}

		fmt.Printf("Added release notes for %v in %v\n", cmd.NextVersion, cmd.ChangelogFile)
	} else {
		// Version section found — update dependency section
		afterVersionHeader := versionIdx + len(versionHeader)

		// Find end of this version's section: next top-level heading (# ) or EOF
		versionEndIdx := len(content)
		remaining := content[afterVersionHeader:]
		searchIdx := 0
		for searchIdx < len(remaining) {
			nlIdx := strings.Index(remaining[searchIdx:], "\n")
			if nlIdx == -1 {
				break
			}
			lineStart := searchIdx + nlIdx + 1
			if lineStart+1 < len(remaining) && remaining[lineStart] == '#' && remaining[lineStart+1] == ' ' {
				versionEndIdx = afterVersionHeader + lineStart
				break
			}
			searchIdx = lineStart
		}

		versionSection := content[afterVersionHeader:versionEndIdx]

		// Find dependency header within version section
		depIdx := strings.Index(versionSection, sdkDependencyHeader)
		if depIdx == -1 {
			panic(fmt.Errorf("unable to find %q section for version %v in %v", sdkDependencyHeader, cmd.NextVersion, cmd.ChangelogFile))
		}

		depAbsIdx := afterVersionHeader + depIdx
		afterDepHeader := depAbsIdx + len(sdkDependencyHeader)

		// Find end of dependency section: next heading (#) or end of version section
		depEndIdx := versionEndIdx
		depRemaining := content[afterDepHeader:versionEndIdx]
		searchIdx = 0
		for searchIdx < len(depRemaining) {
			nlIdx := strings.Index(depRemaining[searchIdx:], "\n")
			if nlIdx == -1 {
				break
			}
			lineStart := searchIdx + nlIdx + 1
			if lineStart < len(depRemaining) && depRemaining[lineStart] == '#' {
				depEndIdx = afterDepHeader + lineStart
				break
			}
			searchIdx = lineStart
		}

		// Generate new dependency content
		var buf bytes.Buffer
		cmd.Writer = &buf
		cmd.generateSdkDependencyUpdates()

		var result strings.Builder
		result.WriteString(content[:depAbsIdx])
		result.WriteString(sdkDependencyHeader)
		result.WriteString("\n\n")
		result.WriteString(buf.String())
		result.WriteString("\n")
		if depEndIdx < len(content) {
			result.WriteString(content[depEndIdx:])
		}

		if err := os.WriteFile(cmd.ChangelogFile, []byte(result.String()), 0644); err != nil {
			panic(fmt.Errorf("unable to write changelog file %v: %w", cmd.ChangelogFile, err))
		}

		fmt.Printf("Updated %q section for %v in %v\n", sdkDependencyHeader, cmd.NextVersion, cmd.ChangelogFile)
	}
}

func newUpdateSdkReleaseNotesCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "update-sdk-release-notes",
		Short: "Builds SDK release notes and adds or updates them in the changelog",
		Args:  cobra.MaximumNArgs(1),
	}

	result := &updateSdkReleaseNotesCmd{
		buildSdkReleaseNotesCmd: buildSdkReleaseNotesCmd{
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
	cobraCmd.Flags().BoolVar(&result.NoPrScan, "no-pr-scan", false, "Don't inspect pull requests for issue links missing from commit messages")
	cobraCmd.Flags().StringVarP(&result.StartVersion, "start-version", "s", "", "Version to diff against, instead of the previous release in the same minor, or the previous minor release")
	cobraCmd.Flags().StringVarP(&result.ChangelogFile, "changelog-file", "c", "CHANGELOG.md", "Path to the changelog file to update")

	return Finalize(result)
}
