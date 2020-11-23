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
	"github.com/netfoundry/ziti-ci/cmd/publish"
	"github.com/spf13/cobra"
	"os"
)

type langType int

const (
	LangGo langType = 1
)

var RootCmd = newRootCommand()

type RootCommand struct {
	RootCobraCmd *cobra.Command

	verbose bool
	dryRun  bool
	quiet   bool

	langName string
	lang     langType

	baseVersionString string
	baseVersionFile   string
}

func newRootCommand() *RootCommand {
	cobraCmd := &cobra.Command{
		Use:   "ziti-ci",
		Short: "Ziti CI Tool",
	}

	var rootCmd = &RootCommand{
		RootCobraCmd: cobraCmd,
	}

	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.verbose, "verbose", "v", false, "enable verbose output")
	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.quiet, "quiet", "q", false, "disable informational output")
	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.dryRun, "dry-run", "d", false, "do a dry run")
	cobraCmd.PersistentFlags().StringVarP(&rootCmd.langName, "language", "l", "go", "enable language specific settings. Valid values: [go]")

	cobraCmd.PersistentFlags().StringVarP(&rootCmd.baseVersionString, "base-version", "b", "", "set base version")
	cobraCmd.PersistentFlags().StringVarP(&rootCmd.baseVersionFile, "base-version-file", "f", DefaultVersionFile, "set base version file location")

	rootCobraCmd := rootCmd.RootCobraCmd

	rootCobraCmd.AddCommand(newTagCmd(rootCmd))
	rootCobraCmd.AddCommand(newGoBuildInfoCmd(rootCmd))
	rootCobraCmd.AddCommand(newConfigureGitCmd(rootCmd))
	rootCobraCmd.AddCommand(newUpdateGoDepCmd(rootCmd))
	rootCobraCmd.AddCommand(newCompleteUpdateGoDepCmd(rootCmd))
	rootCobraCmd.AddCommand(newTriggerJenkinsBuildCmd(rootCmd))
	rootCobraCmd.AddCommand(newTriggerTravisBuildCmd(rootCmd))
	rootCobraCmd.AddCommand(newTriggerGithubBuildCmd(rootCmd))
	rootCobraCmd.AddCommand(newPackageCmd(rootCmd))
	rootCobraCmd.AddCommand(newPublishToArtifactoryCmd(rootCmd))
	rootCobraCmd.AddCommand(newPublishToGithubCmd(rootCmd))
	rootCobraCmd.AddCommand(publish.NewPublishCmd())
	rootCobraCmd.AddCommand(newGetVersionCmd(rootCmd))
	rootCobraCmd.AddCommand(newGetBranchCmd(rootCmd))
	rootCobraCmd.AddCommand(newGetReleaseNotesCmd(rootCmd))

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show build information",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("ziti-ci version: %v, revision: %v, branch: %v, build-by: %v, built-on: %v\n",
				Version, Revision, Branch, BuildUser, BuildDate)
		},
	}

	rootCobraCmd.AddCommand(versionCmd)
	return rootCmd
}

func (r *RootCommand) Execute() {
	if err := r.RootCobraCmd.Execute(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}
