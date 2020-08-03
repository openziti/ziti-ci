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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultGitUsername  = "ziti-ci"
	DefaultGitEmail     = "ziti-ci@netfoundry.io"
	DefaultSshKeyEnvVar = "gh_ci_key"
	DefaultSshKeyFile   = "github_deploy_key"
)

type CiCmd interface {
	GetCobraCmd() *cobra.Command
	Init(args []string)
	Execute()
}

func Finalize(cmd CiCmd) *cobra.Command {
	cmd.GetCobraCmd().Run = func(_ *cobra.Command, args []string) {
		cmd.Init(args)
		cmd.Execute()
	}
	return cmd.GetCobraCmd()
}

type BaseCommand struct {
	*RootCommand

	Cmd  *cobra.Command
	Args []string

	BaseVersion    *version.Version
	CurrentVersion *version.Version
	NextVersion    *version.Version

	CurrentBranch *string
	BuildNumber   *string
}

func (cmd *BaseCommand) Failf(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.Cmd.ErrOrStderr(), format, params...)
	os.Exit(1)
}

func (cmd *BaseCommand) Infof(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.Cmd.OutOrStdout(), format, params...)
}

func (cmd *BaseCommand) Errorf(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.Cmd.OutOrStderr(), format, params...)
}
func (cmd *BaseCommand) Warnf(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.Cmd.OutOrStdout(), "WARNING: " + format, params...)
}

func (cmd *BaseCommand) exitIfErrf(err error, format string, params ...interface{}) {
	if err != nil {
		cmd.Failf(format, params)
	}
}

func (cmd *BaseCommand) isGoLang() bool {
	return cmd.lang == LangGo
}

func (cmd *BaseCommand) getPublishVersion() *version.Version {
	if cmd.CurrentVersion == nil {
		return cmd.NextVersion
	}
	return cmd.CurrentVersion
}

func (cmd *BaseCommand) setLangType() {
	if cmd.langName == "" {
		return
	}
	if strings.EqualFold("go", cmd.langName) {
		cmd.lang = LangGo
	} else {
		cmd.Failf("unsupported language: '%v'\n", cmd.langName)
	}
}

func (cmd *BaseCommand) Init(args []string) {
	cmd.Args = args
	cmd.setLangType()
	cmd.BaseVersion = cmd.getBaseVersion()
}

func (cmd *BaseCommand) GetCobraCmd() *cobra.Command {
	return cmd.Cmd
}

func (cmd *BaseCommand) EvalCurrentAndNextVersion() {
	cmd.runGitCommandAlways("fetching git tags", "fetch", "--tags")
	versions := cmd.getVersionList("tag", "--list")

	min := setPatch(cmd.BaseVersion, 0)
	max := getNext(Minor, min)
	if len(versions) == 0 {
		cmd.NextVersion = min
	}

	for _, v := range versions {
		if cmd.verbose {
			cmd.Infof("Comparing against: %v\n", v)
		}
		if min.LessThanOrEqual(v) && v.LessThan(max) {
			cmd.CurrentVersion = v
		}
	}

	if cmd.CurrentVersion != nil {
		cmd.NextVersion = getNext(Patch, cmd.CurrentVersion)
	} else {
		cmd.NextVersion = min
	}

	if cmd.NextVersion.LessThan(cmd.BaseVersion) {
		cmd.NextVersion = cmd.BaseVersion
	}
	fmt.Printf("current version: %v, next version: %v\n", cmd.CurrentVersion, cmd.NextVersion)
}

func (cmd *BaseCommand) RunGitCommand(description string, params ...string) {
	cmd.runGitCommandOptional(description, cmd.dryRun, params...)
}

func (cmd *BaseCommand) runGitCommandAlways(description string, params ...string) {
	cmd.runGitCommandOptional(description, false, params...)
}

func (cmd *BaseCommand) runGitCommandOptional(description string, dryRun bool, params ...string) {
	cmd.Infof("%v: git %v \n", description, strings.Join(params, " "))
	if !dryRun {
		gitCmd := exec.Command("git", params...)
		gitCmd.Stderr = os.Stderr
		gitCmd.Stdout = os.Stdout
		if err := gitCmd.Run(); err != nil {
			cmd.Failf("error %v: %v\n", description, err)
		}
	}
}

func (cmd *BaseCommand) GetCmdOutputOneLine(description string, name string, params ...string) string {
	output := cmd.runCommandWithOutput(description, name, params...)
	if len(output) != 1 {
		cmd.Failf("expected 1 line return from %v: %v %v, but got %v\n", description, cmd, strings.Join(params, " "), len(output))
	}
	return output[0]
}

func (cmd *BaseCommand) getGoEnv() map[string]string {
	lines := cmd.runCommandWithOutput("get go environment", "go", "env", "-json")
	result := map[string]string{}
	err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &result)
	if err != nil {
		cmd.Failf("error unmarshalling go env json: %v\n", err)
	}
	return result
}

func (cmd *BaseCommand) runCommandWithOutput(description string, name string, params ...string) []string {
	cmd.Infof("%v: %v %v\n", description, name, strings.Join(params, " "))
	command := exec.Command(name, params...)
	command.Stderr = os.Stderr
	output, err := command.Output()
	if err != nil {
		cmd.Failf("error %v: %v\n", description, err)
	}

	stringData := strings.Replace(string(output), "\r\n", "\n", -1)
	lines := strings.Split(stringData, "\n")
	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func (cmd *BaseCommand) runCommand(description string, name string, params ...string) {
	cmd.Infof("%v: %v %v\n", description, name, strings.Join(params, " "))
	command := exec.Command(name, params...)
	command.Stderr = os.Stderr
	command.Stdout = os.Stdout

	if name == "jfrog-cli" {
		command.Env = append(command.Env, "JFROG_CLI_OFFER_CONFIG=false")
	}

	if name != "jfrog-cli" || !cmd.dryRun {
		if err := command.Run(); err != nil {
			cmd.Failf("error %v: %v\n", description, err)
		}
	}
}

func (cmd *BaseCommand) getVersionList(params ...string) []*version.Version {
	lines := cmd.runCommandWithOutput("list git tags", "git", params...)

	var versions []*version.Version

	for _, line := range lines {
		if line == "" {
			continue
		}

		v, err := version.NewVersion(line)
		if err != nil {
			if cmd.verbose {
				cmd.Warnf("failure interpreting tag version on %v: %v\n", line, err)
			}
			continue
		}
		versions = append(versions, v)
		if cmd.verbose {
			cmd.Infof("found version %v\n", v)
		}
	}
	sort.Sort(versionList(versions))
	return versions
}

func (cmd *BaseCommand) getModule() string {
	return cmd.GetCmdOutputOneLine("get go module", "go", "list", "-m")
}

func (cmd *BaseCommand) GetCurrentBranch() string {
	if cmd.CurrentBranch == nil {
		branchName := ""

		if val, found := os.LookupEnv("TRAVIS_PULL_REQUEST_BRANCH"); found && val != "" {
			branchName = val
		} else if val, found := os.LookupEnv("TRAVIS_BRANCH"); found && val != "" {
			branchName = val
		} else {
			branchName = cmd.GetCmdOutputOneLine("get git branch", "git", "rev-parse", "--abbrev-ref", "HEAD")
		}
		cmd.CurrentBranch = &branchName
	}
	return *cmd.CurrentBranch
}

func (cmd *BaseCommand) getBuildNumber() string {
	if cmd.BuildNumber == nil {
		buildNumber := "0"
		if val, found := os.LookupEnv("TRAVIS_BUILD_NUMBER"); found && val != "" {
			buildNumber = val
		}
		cmd.BuildNumber = &buildNumber
	}
	return *cmd.BuildNumber
}

func (cmd *BaseCommand) getCommitterEmail() string {
	return cmd.GetCmdOutputOneLine("get committer e-mail address", "git", "log", "-1", "FETCH_HEAD", "--pretty=%cE")
}

func (cmd *BaseCommand) GetUsername() string {
	currUser, err := user.Current()
	if err != nil {
		cmd.Errorf("unable to get current user %+v\n", err)
		return "unknown"
	}
	return currUser.Name
}

func (cmd *BaseCommand) getBaseVersion() *version.Version {
	if cmd.baseVersionString == "" {
		if cmd.baseVersionFile == "" {
			cmd.baseVersionFile = DefaultVersionFile
		}
		contents, err := ioutil.ReadFile(cmd.baseVersionFile)
		if err != nil {
			currdir, _ := os.Getwd()
			cmd.Errorf("unable to load base version information from '%v'. current dir: '%v'\n", cmd.baseVersionFile, currdir)

			contents, err = ioutil.ReadFile("./common/version/VERSION")
			if err != nil {
				cmd.Failf("unable to load base version information from '%v'. current dir: '%v'\n", cmd.baseVersionFile, currdir)
			}
		}
		cmd.baseVersionString = string(contents)
		cmd.baseVersionString = strings.TrimSpace(cmd.baseVersionString)
	}
	baseVersion, err := version.NewVersion(cmd.baseVersionString)
	if err != nil {
		cmd.Failf("Invalid base version %v\n", cmd.baseVersionString)
	}
	return baseVersion
}

func (cmd *BaseCommand) logJson(data []byte) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "    "); err == nil {
		if _, err := fmt.Printf("Result:\n%s\n", prettyJSON.String()); err != nil {
			panic(err)
		}
	} else {
		if _, err := fmt.Printf("Result:\n%s\n", data); err != nil {
			panic(err)
		}
	}
}

func (cmd *BaseCommand) close(closer io.Closer, descripion string) {
	if err := closer.Close(); err != nil {
		cmd.Errorf("failed to close file %v with err: %v\n", descripion, err)
	}
}

func (cmd *BaseCommand) tarGzSimple(archiveFile string, filesToInclude ...string) {
	nameMap := map[string]string{}
	for _, file := range filesToInclude {
		_, fileName := filepath.Split(file)
		nameMap[file] = fileName
	}
	cmd.tarGz(archiveFile, nameMap)
}

func (cmd *BaseCommand) tarGzArtifacts(archiveFile string, artifacts ...*artifact) {
	nameMap := map[string]string{}
	for _, artifact := range artifacts {
		nameMap[artifact.sourcePath] = fmt.Sprintf("%v/%v/%v", artifact.arch, artifact.os, artifact.sourceName)
	}
	cmd.tarGz(archiveFile, nameMap)
}

func (cmd *BaseCommand) tarGz(archiveFile string, nameMap map[string]string) {
	outputFile, err := os.Create(archiveFile)
	if err != nil {
		cmd.Failf("unexpected err trying to write to %v. err: %+v\n", archiveFile, err)
	}
	gzw := gzip.NewWriter(outputFile)
	defer cmd.close(gzw, "gzip writer for "+archiveFile)

	tw := tar.NewWriter(gzw)
	defer cmd.close(tw, "tar writer for "+archiveFile)

	for filePath, name := range nameMap {
		file, err := os.Open(filePath)
		if err != nil {
			cmd.Failf("unexpected err trying to open file %v. err: %+v\n", filePath, err)
		}
		fileInfo, err := file.Stat()
		if err != nil {
			cmd.close(gzw, "source file "+filePath)
			cmd.Failf("unexpected err trying to read state file %v. err: %+v\n", filePath, err)
		}

		header, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			cmd.close(gzw, "source file "+filePath)
			cmd.Failf("unexpected err trying to create tar header for %v. err: %+v\n", filePath, err)
		}
		header.Name = name
		if err = tw.WriteHeader(header); err != nil {
			cmd.close(gzw, "source file "+filePath)
			cmd.Failf("unexpected err trying to write tar header for %v. err: %+v\n", filePath, err)
		}

		_, err = io.Copy(tw, file)
		cmd.close(file, "source file "+filePath)
		if err != nil {
			cmd.Failf("unexpected err trying to write file %v to tar file. err: %+v\n", filePath, err)
		}
	}
}
