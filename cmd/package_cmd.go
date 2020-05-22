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
	"github.com/spf13/cobra"
)

type packageCmd struct {
	BaseCommand
}

func (cmd *packageCmd) Execute() {
	cmd.tarGzSimple(cmd.Args[0], cmd.Args[1:]...)
}

func newPackageCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "package <destination> <files>",
		Short: "Packages files for release",
		Args:  cobra.MinimumNArgs(2),
	}

	result := &packageCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return Finalize(result)
}
