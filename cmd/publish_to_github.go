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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type publishToGithubCmd struct {
	BaseCommand
	name        string
	archiveBase string
	preRelease  bool
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
	var executableArtifacts []*githubArtifact
	var nonExecutableArtifacts []*githubArtifact

	// Process top-level files in release directory first
	topLevelFiles, err := os.ReadDir(releaseDir)
	cmd.exitIfErrf(err, "failed to read top-level files in release dir: %v\n", err)
	for _, file := range topLevelFiles {
		if !file.IsDir() {
			name := file.Name()
			if name == "checksums.sha256.txt" ||
				(strings.HasPrefix(name, "source-") && strings.HasSuffix(name, ".tar.gz")) ||
				(strings.HasPrefix(name, "sbom-") && strings.HasSuffix(name, ".spdx.json")) {
				filePath := filepath.Join(releaseDir, name)
				nonExecutableArtifacts = append(nonExecutableArtifacts, &githubArtifact{
					name:       name,
					sourceName: name,
					sourcePath: filePath,
				})
			}
		}
	}

	// walk architecture specific subdirs for executables
	for _, archDir := range archDirs {
		if archDir.IsDir() {
			arch := archDir.Name()
			cmd.Infof("processing files for arch: %v\n", arch)
			archDirPath := filepath.Join(releaseDir, archDir.Name())
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

						// Set execute permissions on non-Windows executable files
						if !strings.HasSuffix(releasableFile.Name(), ".exe") {
							if err := os.Chmod(filePath, 0755); err != nil {
								cmd.exitIfErrf(err, "failed to set execute permissions on %v: %v\n", filePath, err)
							}
						}

						executableArtifacts = append(executableArtifacts, &githubArtifact{
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

	for _, artifact := range executableArtifacts {
		bundle := artifact.os + "-" + artifact.arch
		list := bundleMap[bundle]
		list = append(list, artifact)
		bundleMap[bundle] = list
	}

	version := cmd.getPublishVersion().String()

	var releaseArtifacts []string

	// Process architecture-specific executables' bundles
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

	// Add non-executable artifacts to release artifacts
	for _, artifact := range nonExecutableArtifacts {
		releaseArtifacts = append(releaseArtifacts, artifact.sourcePath)
	}

	// Generate checksums file
	checksumFile := filepath.Join(releaseDir, "checksums.sha256.txt")
	checksumWriter, err := os.Create(checksumFile)
	cmd.exitIfErrf(err, "failed to create checksums file: %v\n", err)
	defer checksumWriter.Close()

	// Calculate checksums for all artifacts
	for _, artifactPath := range releaseArtifacts {
		data, err := os.ReadFile(artifactPath)
		cmd.exitIfErrf(err, "failed to read artifact for checksum: %v\n", err)

		hash := sha256.Sum256(data)
		hexHash := hex.EncodeToString(hash[:])

		// Use just the filename for the checksum file
		relPath := filepath.Base(artifactPath)

		_, err = fmt.Fprintf(checksumWriter, "%s  %s\n", hexHash, relPath)
		cmd.exitIfErrf(err, "failed to write checksum: %v\n", err)
	}

	// Add checksums file to release artifacts
	releaseArtifacts = append(releaseArtifacts, checksumFile)

	releaseNotesFile := fmt.Sprintf("changelog-%v.md", version)
	extractReleaseNotes("CHANGELOG.md", version, releaseNotesFile)

	tagName := version
	if cmd.isGoLang() {
		tagName = "v" + version
	}
	releaseParams := []string{"release", "create", tagName, "-F", releaseNotesFile, "--title", tagName}

	if cmd.preRelease {
		releaseParams = append(releaseParams, "--prerelease")
	}

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
	cobraCmd.Flags().BoolVarP(&result.preRelease, "prerelease", "p", false, "Publish as pre-release")
	return Finalize(result)
}
