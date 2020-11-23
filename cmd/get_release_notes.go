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
	"bufio"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
)

type getReleaseNotesCmd struct {
	BaseCommand
}

func extractReleaseNotes(changelog string, version string, outfile string) {
	file, err := os.Open(changelog)
	if err != nil {
		panic(err)
	}
	defer func() { _ = file.Close() }()

	var out io.WriteCloser
	if outfile == "" {
		out = os.Stdout
	} else {
		out, err = os.Create(outfile)
		if err != nil {
			panic(err)
		}
		defer func() { _ = out.Close() }()
	}

	scanner := bufio.NewScanner(file)
	startFound := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# Release") {
			if startFound {
				return
			}
			if version == "" || strings.HasPrefix(line, fmt.Sprintf("# Release %v", version)) {
				startFound = true
			}
		}
		if startFound {
			if _, err := fmt.Fprintln(out, line); err != nil {
				panic(err)
			}
		}
	}
}

func (cmd *getReleaseNotesCmd) Execute() {
	version := ""
	if len(cmd.Args) > 1 {
		version = cmd.Args[1]
	}

	outfile := ""
	if len(cmd.Args) > 2 {
		version = cmd.Args[2]
	}

	extractReleaseNotes(cmd.Args[0], version, outfile)
}

func newGetReleaseNotesCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "get-release-notes <changelog> <version>",
		Short: "Prints out the release notes for the latest or a given version",
		Args:  cobra.RangeArgs(1, 3),
	}

	result := &getReleaseNotesCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return Finalize(result)
}
