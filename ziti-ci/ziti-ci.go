package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	version "github.com/hashicorp/go-version"
)

const (
	Minor = 1
	Patch = 2
)

const (
	DefaultVersionFile = "./common/version/VERSION"
)

type langType int

const (
	LangGo langType = 1
)

var verbose = true
var dryRun = false

var baseVersionString string
var baseVersionFile string

var gitUsername string
var gitEmail string

var langName string
var lang langType

func main() {
	var root = &cobra.Command{
		Use:   "ziti-ci",
		Short: "Ziti CI Tool",
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	root.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "do a dry run")
	root.PersistentFlags().StringVar(&gitUsername, "git-username", "ziti-ci", "override the default git username")
	root.PersistentFlags().StringVar(&gitEmail, "git-email", "ziti-ci@netfoundry.io", "override the default git email")
	root.PersistentFlags().StringVarP(&langName, "language", "l", "go", "enable language specific settings. Valid values: [go]")

	var tag = &cobra.Command{
		Use:   "tag",
		Short: "Tag and push command",
		Run:   runTag,
	}

	tag.PersistentFlags().StringVarP(&baseVersionString, "base-version", "b", "", "set base version")
	tag.PersistentFlags().StringVarP(&baseVersionFile, "base-version-file", "f", DefaultVersionFile, "set base version file location")

	root.AddCommand(tag)

	if err := root.Execute(); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(-1)
	}
}

func setLangType(cmd *cobra.Command) {
	if langName == "" {
		return
	}
	if strings.EqualFold("go", langName) {
		lang = LangGo
	} else {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unsupported language: '%v'\n", langName)
		os.Exit(-1)
	}
}

func runTag(cmd *cobra.Command, args []string) {
	setLangType(cmd)

	if baseVersionString == "" {
		loadBaseVersion(cmd)
	}
	baseVersion, err := version.NewVersion(baseVersionString)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Invalid base version %v\n", baseVersionString)
		os.Exit(-1)
	}

	env := &runEnv{
		cmd:         cmd,
		args:        args,
		baseVersion: baseVersion,
	}

	env.runGitCommand("set git username", "config", "user.name", gitUsername)
	env.runGitCommand("set git password", "config", "user.email", gitEmail)

	env.evalVersions()
	env.ensureNotAlreadyTagged()
	env.evalPrevAndNextVersion()
	_, _ = fmt.Fprintf(cmd.OutOrStderr(), "previous version: %v, next version: %v\n", env.prevVersion, env.nextVersion)

	if lang == LangGo {
		nextMajorVersion := env.nextVersion.Segments()[0]
		if nextMajorVersion > 1 {
			moduleName := env.getModule()
			if !strings.HasSuffix(moduleName, fmt.Sprintf("/v%v", nextMajorVersion)) {
				_, _ = fmt.Fprintf(cmd.OutOrStderr(), "error: module version doesn't match next version: %v\n", nextMajorVersion)
				os.Exit(-1)
			}
		}
	}

	tagVersion := fmt.Sprintf("%v", env.nextVersion)
	if lang == LangGo {
		tagVersion = "v" + tagVersion
	}
	tagParms := []string{"tag", "-a", tagVersion, "-m", fmt.Sprintf("Release %v", tagVersion)}
	env.runGitCommand("create tag", tagParms...)

	// Ensure we're in ssh mode
	if repoSlug, ok := os.LookupEnv("TRAVIS_REPO_SLUG"); ok {
		url := fmt.Sprintf("git@github.com:%v.git", repoSlug)
		env.runGitCommand("set remote to ssh", "remote", "set-url", "origin", url)
	}

	env.runGitCommand("push tag to repo", "push", "origin", tagVersion)
}

type runEnv struct {
	cmd  *cobra.Command
	args []string

	baseVersion *version.Version
	versions    []*version.Version
	headTags    []*version.Version

	prevVersion *version.Version
	nextVersion *version.Version
}

func (env *runEnv) evalPrevAndNextVersion() {
	min := setPatch(env.baseVersion, 0)
	max := getNext(Minor, min)
	if len(env.versions) == 0 {
		env.nextVersion = min
	}

	for _, v := range env.versions {
		if verbose {
			fmt.Printf("Comparing against: %v\n", v)
		}
		if min.LessThanOrEqual(v) && v.LessThan(max) {
			env.prevVersion = v
		}
	}

	if env.prevVersion != nil {
		env.nextVersion = getNext(Patch, env.prevVersion)
	} else {
		env.nextVersion = min
	}

	if env.nextVersion.LessThan(env.baseVersion) {
		env.nextVersion = env.baseVersion
	}
}

func loadBaseVersion(cmd *cobra.Command) {
	if baseVersionFile == "" {
		baseVersionFile = DefaultVersionFile
	}
	contents, err := ioutil.ReadFile(baseVersionFile)
	if err != nil {
		currdir, _ := os.Getwd()
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to load base version information from '%v'. current dir: '%v'\n", baseVersionFile, currdir)
		os.Exit(-1)
	}
	baseVersionString = string(contents)
	baseVersionString = strings.TrimSpace(baseVersionString)
}

func (env *runEnv) evalVersions() {
	env.runGitCommand("fetching git tags", "fetch", "--tags")
	env.versions = env.getVersionList("tag", "--list")
	env.headTags = env.getVersionList("tag", "--points-at", "HEAD")
}

func (env *runEnv) runGitCommand(description string, params ...string) {
	_, _ = fmt.Fprintf(env.cmd.OutOrStderr(), "%v: git %v \n", description, strings.Join(params, " "))
	if !dryRun {
		gitCmd := exec.Command("git", params...)
		gitCmd.Stderr = os.Stderr
		gitCmd.Stdout = os.Stdout
		if err := gitCmd.Run(); err != nil {
			_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "error %v: %v\n", description, err)
			os.Exit(-1)
		}
	}
}

func (env *runEnv) runCommandWithOutput(description string, cmd string, params ...string) []string {
	_, _ = fmt.Fprintf(env.cmd.OutOrStderr(), "%v: %v %v\n", description, cmd, strings.Join(params, " "))
	listTagsCmd := exec.Command(cmd, params...)
	listTagsCmd.Stderr = os.Stderr
	output, err := listTagsCmd.Output()
	if err != nil {
		_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "error %v: %v\n", description, err)
		os.Exit(-1)
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

func (env *runEnv) getVersionList(params ...string) []*version.Version {
	lines := env.runCommandWithOutput("list git tags", "git", params...)

	var versions []*version.Version

	for _, line := range lines {
		if line == "" {
			continue
		}

		v, err := version.NewVersion(line)
		if err != nil && verbose {
			_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "failure interpreting tag version on %v: %v\n", line, err)
			continue
		}
		versions = append(versions, v)
		if verbose {
			_, _ = fmt.Fprintf(env.cmd.OutOrStderr(), "found version %v\n", v)
		}
	}
	sort.Sort(versionList(versions))
	return versions
}

func (env *runEnv) getModule() string {
	lines := env.runCommandWithOutput("get go module", "go", "list", "-m")
	if len(lines) != 1 {
		_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "failure getting go module. Output: %+v\n", lines)
	}
	return lines[0]
}

func (env *runEnv) ensureNotAlreadyTagged() {
	if len(env.headTags) > 0 {
		_, _ = fmt.Fprintf(env.cmd.OutOrStderr(), "head already tagged with %+v:\n", env.headTags)
		os.Exit(0)
	}
}

func setPatch(v *version.Version, patch int) *version.Version {
	parts := v.Segments()
	for len(parts) < 3 {
		parts = append(parts, 0)
	}
	parts[2] = patch
	return newVersion(parts)
}

func getNext(index int, v *version.Version) *version.Version {
	parts := v.Segments()
	for len(parts) < 3 && len(parts) < (index+1) {
		parts = append(parts, 0)
	}
	parts[index] = parts[index] + 1
	return newVersion(parts)
}

func newVersion(parts []int) *version.Version {
	var stringParts []string
	for _, part := range parts {
		stringParts = append(stringParts, strconv.Itoa(part))
	}
	versionString := strings.Join(stringParts, ".")
	result, err := version.NewVersion(versionString)
	if err != nil {
		panic(err)
	}
	return result
}

type versionList []*version.Version

func (list versionList) Len() int {
	return len(list)
}

func (list versionList) Less(i, j int) bool {
	return list[i].Compare(list[j]) < 0
}

func (list versionList) Swap(i, j int) {
	tmp := list[i]
	list[i] = list[j]
	list[j] = tmp
}
