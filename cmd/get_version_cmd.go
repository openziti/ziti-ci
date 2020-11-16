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
)

type getVersionCmd struct {
	BaseCommand
}

func (cmd *getVersionCmd) Execute() {
	cmd.EvalCurrentAndNextVersion()

	headTags := cmd.getVersionList("tag", "--points-at", "HEAD")
	if len(headTags) > 0 {
		cmd.Errorf("head already tagged with %+v:\n", headTags)
		os.Exit(0)
	}

	tagVersion := fmt.Sprintf("%v", cmd.NextVersion)
	if cmd.isGoLang() {
		tagVersion = "v" + tagVersion
	}
	fmt.Print(tagVersion)
}

func newGetVersionCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "get-version",
		Short: "Print out the version being built. Should be run before tag is actually made",
		Args:  cobra.ExactArgs(0),
	}

	result := &getVersionCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return Finalize(result)
}
