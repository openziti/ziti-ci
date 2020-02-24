package main

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

type ciCmd interface {
	getCobraCmd() *cobra.Command
	init(args []string)
	execute()
}

func finalize(cmd ciCmd) *cobra.Command {
	cmd.getCobraCmd().Run = func(_ *cobra.Command, args []string) {
		cmd.init(args)
		cmd.execute()
	}
	return cmd.getCobraCmd()
}

type baseCommand struct {
	*rootCommand

	cmd  *cobra.Command
	args []string

	baseVersion    *version.Version
	currentVersion *version.Version
	nextVersion    *version.Version

	currentBranch *string
	buildNumber   *string
}

func (cmd *baseCommand) failf(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.cmd.ErrOrStderr(), format, params...)
	os.Exit(-1)
}

func (cmd *baseCommand) infof(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.cmd.OutOrStdout(), format, params...)
}

func (cmd *baseCommand) errorf(format string, params ...interface{}) {
	_, _ = fmt.Fprintf(cmd.cmd.OutOrStderr(), format, params...)
}

func (cmd *baseCommand) exitIfErrf(err error, format string, params ...interface{}) {
	if err != nil {
		cmd.failf(format, params)
	}
}

func (cmd *baseCommand) isGoLang() bool {
	return cmd.lang == LangGo
}

func (cmd *baseCommand) setLangType() {
	if cmd.langName == "" {
		return
	}
	if strings.EqualFold("go", cmd.langName) {
		cmd.lang = LangGo
	} else {
		cmd.failf("unsupported language: '%v'\n", cmd.langName)
	}
}

func (cmd *baseCommand) init(args []string) {
	cmd.args = args
	cmd.setLangType()
	cmd.baseVersion = cmd.getBaseVersion()
}

func (cmd *baseCommand) getCobraCmd() *cobra.Command {
	return cmd.cmd
}

func (cmd *baseCommand) evalCurrentAndNextVersion() {
	cmd.runGitCommand("fetching git tags", "fetch", "--tags")
	versions := cmd.getVersionList("tag", "--list")

	min := setPatch(cmd.baseVersion, 0)
	max := getNext(Minor, min)
	if len(versions) == 0 {
		cmd.nextVersion = min
	}

	for _, v := range versions {
		if cmd.verbose {
			cmd.infof("Comparing against: %v\n", v)
		}
		if min.LessThanOrEqual(v) && v.LessThan(max) {
			cmd.currentVersion = v
		}
	}

	if cmd.currentVersion != nil {
		cmd.nextVersion = getNext(Patch, cmd.currentVersion)
	} else {
		cmd.nextVersion = min
	}

	if cmd.nextVersion.LessThan(cmd.baseVersion) {
		cmd.nextVersion = cmd.baseVersion
	}
}

func (cmd *baseCommand) runGitCommand(description string, params ...string) {
	cmd.infof("%v: git %v \n", description, strings.Join(params, " "))
	if !cmd.dryRun {
		gitCmd := exec.Command("git", params...)
		gitCmd.Stderr = os.Stderr
		gitCmd.Stdout = os.Stdout
		if err := gitCmd.Run(); err != nil {
			cmd.failf("error %v: %v\n", description, err)
		}
	}
}

func (cmd *baseCommand) getCmdOutputOneLine(description string, name string, params ...string) string {
	output := cmd.runCommandWithOutput(description, name, params...)
	if len(output) != 1 {
		cmd.failf("expected 1 line return from %v: %v %v, but got %v\n", description, cmd, strings.Join(params, " "), len(output))
	}
	return output[0]
}

func (cmd *baseCommand) getGoEnv() map[string]string {
	lines := cmd.runCommandWithOutput("get go environment", "go", "env", "-json")
	result := map[string]string{}
	err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &result)
	if err != nil {
		cmd.failf("error unmarshalling go env json: %v\n", err)
	}
	return result
}

func (cmd *baseCommand) runCommandWithOutput(description string, name string, params ...string) []string {
	cmd.infof("%v: %v %v\n", description, name, strings.Join(params, " "))
	command := exec.Command(name, params...)
	command.Stderr = os.Stderr
	output, err := command.Output()
	if err != nil {
		cmd.failf("error %v: %v\n", description, err)
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

func (cmd *baseCommand) runCommand(description string, name string, params ...string) {
	cmd.infof("%v: %v %v\n", description, name, strings.Join(params, " "))
	command := exec.Command(name, params...)
	command.Stderr = os.Stderr
	command.Stdout = os.Stdout

	if err := command.Run(); err != nil {
		cmd.failf("error %v: %v\n", description, err)
	}
}

func (cmd *baseCommand) getVersionList(params ...string) []*version.Version {
	lines := cmd.runCommandWithOutput("list git tags", "git", params...)

	var versions []*version.Version

	for _, line := range lines {
		if line == "" {
			continue
		}

		v, err := version.NewVersion(line)
		if err != nil && cmd.verbose {
			cmd.errorf("failure interpreting tag version on %v: %v\n", line, err)
			continue
		}
		versions = append(versions, v)
		if cmd.verbose {
			cmd.infof("found version %v\n", v)
		}
	}
	sort.Sort(versionList(versions))
	return versions
}

func (cmd *baseCommand) getModule() string {
	return cmd.getCmdOutputOneLine("get go module", "go", "list", "-m")
}

func (cmd *baseCommand) getCurrentBranch() string {
	if cmd.currentBranch == nil {
		branchName := ""

		if val, found := os.LookupEnv("TRAVIS_PULL_REQUEST_BRANCH"); found && val != "" {
			branchName = val
		} else if val, found := os.LookupEnv("TRAVIS_BRANCH"); found && val != "" {
			branchName = val
		} else {
			branchName = cmd.getCmdOutputOneLine("get git branch", "git", "rev-parse", "--abbrev-ref", "HEAD")
		}
		cmd.currentBranch = &branchName
	}
	return *cmd.currentBranch
}

func (cmd *baseCommand) getBuildNumber() string {
	if cmd.buildNumber == nil {
		buildNumber := "0"
		if val, found := os.LookupEnv("TRAVIS_BUILD_NUMBER"); found && val != "" {
			buildNumber = val
		}
		cmd.buildNumber = &buildNumber
	}
	return *cmd.buildNumber
}

func (cmd *baseCommand) getUsername() string {
	currUser, err := user.Current()
	if err != nil {
		cmd.errorf("unable to get current user %+v\n", err)
		return "unknown"
	}
	return currUser.Name
}

func (cmd *baseCommand) getBaseVersion() *version.Version {
	if cmd.baseVersionString == "" {
		if cmd.baseVersionFile == "" {
			cmd.baseVersionFile = DefaultVersionFile
		}
		contents, err := ioutil.ReadFile(cmd.baseVersionFile)
		if err != nil {
			currdir, _ := os.Getwd()
			cmd.errorf("unable to load base version information from '%v'. current dir: '%v'\n", cmd.baseVersionFile, currdir)

			contents, err = ioutil.ReadFile("./common/version/VERSION")
			if err != nil {
				cmd.failf("unable to load base version information from '%v'. current dir: '%v'\n", cmd.baseVersionFile, currdir)
			}
		}
		cmd.baseVersionString = string(contents)
		cmd.baseVersionString = strings.TrimSpace(cmd.baseVersionString)
	}
	baseVersion, err := version.NewVersion(cmd.baseVersionString)
	if err != nil {
		cmd.failf("Invalid base version %v\n", cmd.baseVersionString)
	}
	return baseVersion
}

func (cmd *baseCommand) logJson(data []byte) {
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

func (cmd *baseCommand) close(closer io.Closer, descripion string) {
	if err := closer.Close(); err != nil {
		cmd.errorf("failed to close file %v with err: %v\n", descripion, err)
	}
}

func (cmd *baseCommand) tarGzSimple(archiveFile string, filesToInclude ...string) {
	nameMap := map[string]string{}
	for _, file := range filesToInclude {
		_, fileName := filepath.Split(file)
		nameMap[file] = fileName
	}
	cmd.tarGz(archiveFile, nameMap)
}

func (cmd *baseCommand) tarGzArtifacts(archiveFile string, artifacts ...*artifact) {
	nameMap := map[string]string{}
	for _, artifact := range artifacts {
		nameMap[artifact.artifactPath] = fmt.Sprintf("%v/%v/%v", artifact.arch, artifact.os, artifact.artifactArchive)
	}
	cmd.tarGz(archiveFile, nameMap)
}

func (cmd *baseCommand) tarGz(archiveFile string, nameMap map[string]string) {
	outputFile, err := os.Create(archiveFile)
	if err != nil {
		cmd.failf("unexpected err trying to write to %v. err: %+v\n", archiveFile, err)
	}
	gzw := gzip.NewWriter(outputFile)
	defer cmd.close(gzw, "gzip writer for "+archiveFile)

	tw := tar.NewWriter(gzw)
	defer cmd.close(tw, "tar writer for "+archiveFile)

	for filePath, name := range nameMap {
		file, err := os.Open(filePath)
		if err != nil {
			cmd.failf("unexpected err trying to open file %v. err: %+v\n", filePath, err)
		}
		fileInfo, err := file.Stat()
		if err != nil {
			cmd.close(gzw, "source file "+filePath)
			cmd.failf("unexpected err trying to read state file %v. err: %+v\n", filePath, err)
		}

		header, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			cmd.close(gzw, "source file "+filePath)
			cmd.failf("unexpected err trying to create tar header for %v. err: %+v\n", filePath, err)
		}
		header.Name = name
		if err = tw.WriteHeader(header); err != nil {
			cmd.close(gzw, "source file "+filePath)
			cmd.failf("unexpected err trying to write tar header for %v. err: %+v\n", filePath, err)
		}

		_, err = io.Copy(tw, file)
		cmd.close(file, "source file "+filePath)
		if err != nil {
			cmd.failf("unexpected err trying to write file %v to tar file. err: %+v\n", filePath, err)
		}
	}
}
