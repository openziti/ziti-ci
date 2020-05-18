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

package main

import (
	"github.com/spf13/cobra"
)

type langType int

const (
	LangGo langType = 1
)

type rootCommand struct {
	rootCobraCmd *cobra.Command

	verbose bool
	dryRun  bool

	langName string
	lang     langType

	baseVersionString string
	baseVersionFile   string
}

func newRootCommand() *rootCommand {
	cobraCmd := &cobra.Command{
		Use:   "ziti-ci",
		Short: "Ziti CI Tool",
	}

	var rootCmd = &rootCommand{
		rootCobraCmd: cobraCmd,
	}

	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.verbose, "verbose", "v", false, "enable verbose output")
	cobraCmd.PersistentFlags().BoolVarP(&rootCmd.dryRun, "dry-run", "d", false, "do a dry run")
	cobraCmd.PersistentFlags().StringVarP(&rootCmd.langName, "language", "l", "go", "enable language specific settings. Valid values: [go]")

	cobraCmd.PersistentFlags().StringVarP(&rootCmd.baseVersionString, "base-version", "b", "", "set base version")
	cobraCmd.PersistentFlags().StringVarP(&rootCmd.baseVersionFile, "base-version-file", "f", DefaultVersionFile, "set base version file location")

	return rootCmd
}
