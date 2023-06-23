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
	"path/filepath"
	"strings"
)

type publishToGithubCmd struct {
	BaseCommand
	name        string
	archiveBase string
}

type githubArtifact struct {
	name       string
	sourceName string
	sourcePath string
	arch       string
	os         string
}

func (cmd *publishToGithubCmd) Execute() {
	cmd.name = "ziti"
	if len(cmd.Args) > 0 {
		cmd.name = cmd.Args[0]
	}

	if !cmd.Cmd.Flags().Changed("archive-base") {
		cmd.archiveBase = cmd.name
	}

	cmd.EvalCurrentAndNextVersion()

	releaseDir, err := filepath.Abs("./release")
	cmd.exitIfErrf(err, "could not get absolute path for releases directory")

	archDirs, err := os.ReadDir(releaseDir)
	cmd.exitIfErrf(err, "failed to read releases dir: %v\n", err)
	var artifacts []*githubArtifact
	for _, archDir := range archDirs {
		arch := archDir.Name()
		cmd.Infof("processing files for arch: %v\n", arch)
		archDirPath := filepath.Join(releaseDir, archDir.Name())

		if archDir.IsDir() {
			osDirs, err := os.ReadDir(archDirPath)
			cmd.exitIfErrf(err, "failed to read arch dir %v: %v\n", archDirPath, err)

			for _, osDir := range osDirs {
				osName := osDir.Name()
				cmd.Infof("processing files for: %v/%v\n", arch, osName)

				osDirPath := filepath.Join(archDirPath, osDir.Name())
				releasableFiles, err := os.ReadDir(osDirPath)
				cmd.exitIfErrf(err, "failed to read os dir %v: %v\n", osDirPath, err)

				for _, releasableFile := range releasableFiles {
					if !releasableFile.IsDir() && !strings.HasSuffix(releasableFile.Name(), ".gz") {
						name := releasableFile.Name()
						if strings.HasSuffix(name, ".exe") {
							name = strings.TrimSuffix(name, ".exe")
						}
						filePath := filepath.Join(osDirPath, releasableFile.Name())
						artifacts = append(artifacts, &githubArtifact{
							name:       name,
							sourceName: releasableFile.Name(),
							sourcePath: filePath,
							arch:       arch,
							os:         osName,
						})
					}
				}
			}
		}
	}

	bundleMap := map[string][]*githubArtifact{}

	for _, artifact := range artifacts {
		bundle := artifact.os + "-" + artifact.arch
		list := bundleMap[bundle]
		list = append(list, artifact)
		bundleMap[bundle] = list
	}

	version := cmd.getPublishVersion().String()

	var releaseArtifacts []string

	for k, v := range bundleMap {
		if strings.Contains(k, "windows") {
			file := fmt.Sprintf("release/%v-%v-%v.zip", cmd.name, k, version)
			cmd.Infof("Creating release archive %v\n", file)
			cmd.zipGhArtifacts(cmd.name, file, v...)
			releaseArtifacts = append(releaseArtifacts, file)
		} else {
			file := fmt.Sprintf("release/%v-%v-%v.tar.gz", cmd.name, k, version)
			cmd.Infof("Creating release archive %v\n", file)
			cmd.tarGzGhArtifacts(cmd.name, file, v...)
			releaseArtifacts = append(releaseArtifacts, file)
		}
	}

	releaseNotesFile := fmt.Sprintf("changelog-%v.md", version)
	extractReleaseNotes("CHANGELOG.md", version, releaseNotesFile)

	tagName := version
	if cmd.isGoLang() {
		tagName = "v" + version
	}
	releaseParams := []string{"release", "create", tagName, "-F", releaseNotesFile}

	for _, releaseArtifact := range releaseArtifacts {
		cmd.Infof("Publishing %v\n", releaseArtifact)
		releaseParams = append(releaseParams, releaseArtifact)
	}

	if !cmd.dryRun {
		cmd.runCommand("Create GH Release and publish release artifacts", "gh", releaseParams...)
	}
}

func newPublishToGithubCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "publish-to-github <name>",
		Short: "Creates archives to be published",
		Args:  cobra.RangeArgs(0, 1),
	}

	result := &publishToGithubCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.Flags().StringVar(&result.archiveBase, "archive-base", "", "Directory to store release files in archives defaults to project name if not specified. May be set to blank.")

	return Finalize(result)
}
