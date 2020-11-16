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

type getBranchCmd struct {
	BaseCommand
}

func (cmd *getBranchCmd) Execute() {
	fmt.Print(cmd.GetCurrentBranch())
}

func newGetBranchCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "get-branch",
		Short: "Print out the git branch being built",
		Args:  cobra.ExactArgs(0),
	}

	result := &getBranchCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return Finalize(result)
}
