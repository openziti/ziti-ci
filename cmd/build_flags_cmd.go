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
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

type GoBuildFlagsCmd struct {
	BaseCommand
	nextVersion bool
}

func (cmd *GoBuildFlagsCmd) Execute() {
	cmd.EvalCurrentAndNextVersion()

	tagVersion := fmt.Sprintf("v%s", cmd.CurrentVersion)
	if cmd.nextVersion {
		tagVersion = fmt.Sprintf("v%v", cmd.NextVersion)
	}

	revision := cmd.GetCmdOutputOneLine("get git SHA", "git", "rev-parse", "--short=12", "HEAD")
	buildDate := time.Now().Format(time.RFC3339)

	data, err := os.ReadFile("go.mod")
	if err != nil {
		panic(err)
	}

	newGoMod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		panic(err)
	}

	modulePath := newGoMod.Module.Mod.Path
	buildInfoPath := "common/version"

	versionFlag := fmt.Sprintf("%s/%s.Version=%s", modulePath, buildInfoPath, tagVersion)
	revisionFlag := fmt.Sprintf("%s/%s.Revision=%s", modulePath, buildInfoPath, revision)
	buildDateFlag := fmt.Sprintf("%s/%s.BuildDate=%s", modulePath, buildInfoPath, buildDate)
	fmt.Printf(`-X '%s' -X '%s' -X '%s'`, versionFlag, revisionFlag, buildDateFlag)
}

func newGoBuildFlagsCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "go-build-flags",
		Short: "Outputs go build flags",
		Args:  cobra.MaximumNArgs(0),
	}

	result := &GoBuildFlagsCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.Flags().BoolVarP(&result.nextVersion, "next-version", "n", false, "use the next version instead of the current version")

	return Finalize(result)
}
