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
)

var knownZitiVersions = []string{
	"v1.1.3",
	"v1.1.2",
	"v1.1.1",
	"v1.1.0",
	"v1.0.0",

	"v0.34.3",
	"v0.34.2",
	"v0.34.1",
	"v0.34.0",

	"v0.33.1",
	"v0.33.0",

	"v0.32.2",
	"v0.32.1",
	"v0.32.0",

	"v0.31.4",
	"v0.31.3",
	"v0.31.2",
	"v0.31.1",
	"v0.31.0",

	"v0.30.5",
	"v0.30.4",
	"v0.30.3",
	"v0.30.2",
	"v0.30.1",
	"v0.30.0",

	"v0.29.0",

	"v0.28.4",
	"v0.28.3",
	"v0.28.2",
	"v0.28.1",
	"v0.28.0",

	"v0.27.9",
	"v0.27.8",
	"v0.27.7",
	"v0.27.6",
	"v0.27.5",
	"v0.27.4",
	"v0.27.3",
	"v0.27.2",
	"v0.27.1",
	"v0.27.0",

	"v0.26.11",
	"v0.26.10",
	"v0.26.9",
	"v0.26.8",
	"v0.26.7",
	"v0.26.6",
	"v0.26.5",
	"v0.26.4",
	"v0.26.3",
	"v0.26.2",
	"v0.26.1",
	"v0.26.0",

	"v0.25.13",
	"v0.25.12",
	"v0.25.11",
	"v0.25.10",
	"v0.25.9",
	"v0.25.8",
	"v0.25.7",
	"v0.25.6",
	"v0.25.5",
	"v0.25.4",
	"v0.25.3",
	"v0.25.2",
	"v0.25.1",
	"v0.25.0",

	"v0.24.13",
	"v0.24.12",
	"v0.24.11",
	"v0.24.10",
	"v0.24.9",
	"v0.24.8",
	"v0.24.7",
	"v0.24.6",
	"v0.24.5",
	"v0.24.4",
	"v0.24.3",
	"v0.24.2",
	"v0.24.1",
	"v0.24.0",

	"v0.23.1",
	"v0.23.0",

	"v0.22.11",
	"v0.22.10",
	"v0.22.9",
	"v0.22.8",
	"v0.22.7",
	"v0.22.6",
	"v0.22.5",
	"v0.22.4",
	"v0.22.3",
	"v0.22.2",
	"v0.22.1",
	"v0.22.0",

	"v0.21.0",

	"v0.20.14",
	"v0.20.13",
	"v0.20.12",
	"v0.20.11",
	"v0.20.10",
	"v0.20.9",
	"v0.20.8",
	"v0.20.7",
	"v0.20.6",
	"v0.20.5",
	"v0.20.4",
	"v0.20.3",
	"v0.20.2",
	"v0.20.1",
	"v0.20.0",

	"v0.19.13",
	"v0.19.12",
	"v0.19.11",
	"v0.19.10",
	"v0.19.9",
	"v0.19.8",
	"v0.19.7",
	"v0.19.6",
	"v0.19.5",
	"v0.19.4",
	"v0.19.3",
	"v0.19.2",
	"v0.19.1",
	"v0.19.0",

	"v0.18.10",
	"v0.18.9",
	"v0.18.8",
	"v0.18.7",
	"v0.18.6",
	"v0.18.5",
	"v0.18.4",
	"v0.18.3",
	"v0.18.2",
	"v0.18.1",
	"v0.18.0",

	"v0.9.0",
	"v0.9.1",
	"v0.9.2",
	"v0.9.3",
	"v0.9.4",
	"v0.9.5",
	"v0.9.6",
	"v0.9.7",
	"v0.9.8",
	"v0.9.9",

	"v0.10.0",
	"v0.10.1",

	"v0.11.0",
	"v0.11.1",
	"v0.11.2",
	"v0.11.3",
	"v0.11.4",
	"v0.11.5",
	"v0.11.6",
	"v0.11.7",

	"v0.12.0",
	"v0.12.1",
	"v0.12.2",
	"v0.12.3",
	"v0.12.4",

	"v0.13.0",
	"v0.13.1",
	"v0.13.2",
	"v0.13.3",
	"v0.13.4",
	"v0.13.5",
	"v0.13.6",
	"v0.13.7",
	"v0.13.8",
	"v0.13.9",
	"v0.13.10",

	"v0.14.0",
	"v0.14.1",
	"v0.14.2",
	"v0.14.3",
	"v0.14.4",
	"v0.14.5",
	"v0.14.6",
	"v0.14.7",
	"v0.14.8",
	"v0.14.9",
	"v0.14.10",
	"v0.14.11",
	"v0.14.12",
	"v0.14.13",
	"v0.14.14",

	"v0.15.0",
	"v0.15.1",
	"v0.15.2",
	"v0.15.3",

	"v0.16.0",
	"v0.16.1",
	"v0.16.2",
	"v0.16.3",
	"v0.16.4",
	"v0.16.5",

	"v0.17.0",
	"v0.17.1",
	"v0.17.2",
	"v0.17.3",
	"v0.17.4",
	"v0.17.5",
	"v0.17.6",
	"v0.17.7",
	"v0.17.8",
}

type TidyTagsCmd struct {
	BaseCommand
	fix bool
}

func (cmd *TidyTagsCmd) Execute() {
	versions := cmd.getVersionList("tag", "--list")
	versionMap := map[string]struct{}{}
	for _, version := range knownZitiVersions {
		versionMap[version] = struct{}{}
	}

	for _, version := range versions {
		if _, found := versionMap["v"+version.String()]; found {
			continue
		}
		//parts := version.Segments()
		//if parts[0] == 0 && parts[1] < 30 {
		fmt.Printf("tag v%s\n", version.String())
		if cmd.fix {
			cmd.RunGitCommand("delete remote tag", "push", "origin", "--delete", fmt.Sprintf("v%s", version.String()))
			cmd.RunGitCommand("delete local tag", "tag", "--delete", fmt.Sprintf("v%s", version.String()))
		}
		//}
	}
}

func newTidyTagsCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "tidy-tags",
		Short: "Outputs go build flags",
		Args:  cobra.MaximumNArgs(0),
	}

	result := &TidyTagsCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.Flags().BoolVar(&result.fix, "fix", false, "fix merged tags")

	return Finalize(result)
}
