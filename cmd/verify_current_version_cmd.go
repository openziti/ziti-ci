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

type verifyCurrentVersionCmd struct {
	BaseCommand
}

func (cmd *verifyCurrentVersionCmd) Execute() error {
	cmd.EvalCurrentAndNextVersion()

	tagVersion := fmt.Sprintf("%v", cmd.CurrentVersion)
	if cmd.isGoLang() {
		tagVersion = "v" + tagVersion
	}
	if cmd.Args[0] != tagVersion {
		return fmt.Errorf("version check failed: expected %v, got %v", tagVersion, cmd.Args[0])
	}
	return nil
}

func newVerifyCurrentVersionCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "verify-current-version",
		Short: "Verify that version passed in matches the current tag",
		Args:  cobra.ExactArgs(1),
	}

	result := &verifyCurrentVersionCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	return FinalizeErroringCmd(result)
}
