package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type publishToArtifactoryCmd struct {
	baseCommand
}

type artifact struct {
	name            string
	artifactArchive string
	sourceName      string
	sourcePath      string
	artifactPath    string
	arch            string
	os              string
}

func (cmd *publishToArtifactoryCmd) execute() {
	jfrogApiKey, found := os.LookupEnv("JFROG_API_KEY")
	if !found {
		cmd.failf("JFROG_API_KEY not specified")
	}

	cmd.evalCurrentAndNextVersion()

	cmd.runCommand("install jfrog cli", "go", "get", "github.com/jfrog/jfrog-cli-go/...")
	releaseDir, err := filepath.Abs("./release")
	cmd.exitIfErrf(err, "could not get absolute path for releases directory")

	archDirs, err := ioutil.ReadDir(releaseDir)
	cmd.exitIfErrf(err, "failed to read releases dir: %v\n", err)
	var artifacts []*artifact
	for _, archDir := range archDirs {
		arch := archDir.Name()
		cmd.infof("processing files for arch: %v\n", arch)
		archDirPath := filepath.Join(releaseDir, archDir.Name())

		if archDir.IsDir() {
			osDirs, err := ioutil.ReadDir(archDirPath)
			cmd.exitIfErrf(err, "failed to read arch dir %v: %v\n", archDirPath, err)

			for _, osDir := range osDirs {
				os := osDir.Name()
				cmd.infof("processing files for: %v/%v\n", arch, os)

				osDirPath := filepath.Join(archDirPath, osDir.Name())
				releasableFiles, err := ioutil.ReadDir(osDirPath)
				cmd.exitIfErrf(err, "failed to read os dir %v: %v\n", osDirPath, err)

				for _, releasableFile := range releasableFiles {
					if !releasableFile.IsDir() && !strings.HasSuffix(releasableFile.Name(), ".gz") {
						name := releasableFile.Name()
						if strings.HasSuffix(name, ".exe") {
							name = strings.TrimSuffix(name, ".exe")
						}
						filePath := filepath.Join(osDirPath, releasableFile.Name())
						destPath := filepath.Join(osDirPath, name+".tar.gz")
						cmd.infof("packaging releasable: %v -> %v\n", filePath, destPath)
						cmd.tarGzSimple(destPath, filePath)
						artifacts = append(artifacts, &artifact{
							name:            name,
							sourceName:      releasableFile.Name(),
							sourcePath:      filePath,
							artifactArchive: name + ".tar.gz",
							artifactPath:    destPath,
							arch:            arch,
							os:              os,
						})
					}
				}
			}
		}
	}

	zitiAllPath := "release/ziti-all.tar.gz"
	cmd.tarGzArtifacts(zitiAllPath, artifacts...)

	// When rolling minor/major numbers the current version will be nil, so use the next version instead
	// This will only happen when publishing a PR
	version := cmd.nextVersion.String()
	if cmd.currentVersion != nil {
		version = cmd.currentVersion.String()
	}

	if cmd.getCurrentBranch() != "master" {
		version = fmt.Sprintf("%v-%v", cmd.currentVersion, cmd.getBuildNumber())
	}

	for _, artifact := range artifacts {
		dest := ""
		// if release branch, publish to staging, otherwise to snapshot
		if cmd.getCurrentBranch() == "master" {
			dest = fmt.Sprintf("ziti-staging/%v/%v/%v/%v/%v",
				artifact.name, artifact.arch, artifact.os, version, artifact.artifactArchive)
		} else {
			dest = fmt.Sprintf("ziti-snapshot/%v/%v/%v/%v/%v/%v",
				cmd.getCurrentBranch(), artifact.name, artifact.arch, artifact.os, version, artifact.artifactArchive)
		}
		props := fmt.Sprintf("version=%v;name=%v;arch=%v;os=%v;branch=%v", version, artifact.name, artifact.arch, artifact.os, cmd.getCurrentBranch())
		cmd.runCommand(fmt.Sprintf("Publish artifact for %v", artifact.name),
			"jfrog", "rt", "u", artifact.artifactPath, dest,
			"--apikey", jfrogApiKey,
			"--url", "https://netfoundry.jfrog.io/netfoundry",
			"--props", props,
			"--build-name=ziti",
			"--build-number="+cmd.currentVersion.String())
	}

	if cmd.getCurrentBranch() == "master" {
		dest := fmt.Sprintf("ziti-staging/ziti-all/%v/ziti-all.%v.tar.gz", version, version)
		props := fmt.Sprintf("version=%v;branch=%v", version, cmd.getCurrentBranch())
		cmd.runCommand("Publish artifact for ziti-all",
			"jfrog", "rt", "u", zitiAllPath, dest,
			"--apikey", jfrogApiKey,
			"--url", "https://netfoundry.jfrog.io/netfoundry",
			"--props", props,
			"--build-name=ziti",
			"--build-number="+cmd.currentVersion.String())
	}
}

func newPublishToArtifactoryCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "publish-to-artifactory",
		Short: "Publishes an artifact to artifactory",
		Args:  cobra.ExactArgs(0),
	}

	result := &publishToArtifactoryCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	return finalize(result)
}
