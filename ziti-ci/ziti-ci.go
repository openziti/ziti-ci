package main

import (
	"encoding/base64"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"os/user"
	"strings"
	"text/template"
	"time"

	version "github.com/hashicorp/go-version"
)

const (
	Minor = 1
	Patch = 2
)

const (
	DefaultVersionFile  = "./version"
	DefaultSshKeyEnvVar = "gh-ci-key"
	DefaultSshKeyFile   = "github_deploy_key"
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

var sshKeyEnv string
var sshKeyFile string

func main() {
	var rootCmd = &cobra.Command{
		Use:   "ziti-ci",
		Short: "Ziti CI Tool",
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "d", false, "do a dry run")
	rootCmd.PersistentFlags().StringVar(&gitUsername, "git-username", "ziti-ci", "override the default git username")
	rootCmd.PersistentFlags().StringVar(&gitEmail, "git-email", "ziti-ci@netfoundry.io", "override the default git email")
	rootCmd.PersistentFlags().StringVarP(&langName, "language", "l", "go", "enable language specific settings. Valid values: [go]")

	var tagCmd = &cobra.Command{
		Use:   "tag",
		Short: "Tag and push command",
		Run:   runTag,
	}

	tagCmd.PersistentFlags().StringVarP(&baseVersionString, "base-version", "b", "", "set base version")
	tagCmd.PersistentFlags().StringVarP(&baseVersionFile, "base-version-file", "f", DefaultVersionFile, "set base version file location")

	rootCmd.AddCommand(tagCmd)

	var buildInfoCmd = &cobra.Command{
		Use:   "generate-build-info output-file go-package",
		Short: "Tag and push command",
		Args:  cobra.ExactArgs(2),
		Run:   generateBuildInfo,
	}

	rootCmd.AddCommand(buildInfoCmd)

	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show build information",
		Args:  cobra.ExactArgs(0),
		Run:   showBuildInfo,
	}

	rootCmd.AddCommand(versionCmd)

	var setupGitCmd = &cobra.Command{
		Use:   "configure-git",
		Short: "Configure git",
		Args:  cobra.ExactArgs(0),
		Run:   configureGit,
	}

	setupGitCmd.PersistentFlags().StringVar(&sshKeyEnv, "ssh-key-env-var", DefaultSshKeyEnvVar, "set ssh key environment variable name")
	setupGitCmd.PersistentFlags().StringVar(&sshKeyFile, "ssh-key-file", DefaultSshKeyFile, "set ssh key file name")

	rootCmd.AddCommand(setupGitCmd)

	if err := rootCmd.Execute(); err != nil {
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

func newRunEnv(cmd *cobra.Command, args []string) *runEnv {
	setLangType(cmd)

	env := &runEnv{
		cmd:         cmd,
		args:        args,
		baseVersion: getBaseVersion(cmd),
	}

	return env
}

func runTag(cmd *cobra.Command, args []string) {
	env := newRunEnv(cmd, args)
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

var buildInfoTemplate = `// Code generated by ziti-ci. DO NOT EDIT.

package {{.PackageName}}

const (
	Version   = "{{.Version}}"
	Revision  = "{{.Revision}}"
	Branch    = "{{.Branch}}"
	BuildUser = "{{.BuildUser}}"
	BuildDate = "{{.BuildDate}}"
)
`

type BuildInfo struct {
	PackageName string
	Version     string
	Revision    string
	Branch      string
	BuildUser   string
	BuildDate   string
}

func generateBuildInfo(cmd *cobra.Command, args []string) {
	env := newRunEnv(cmd, args)
	env.evalVersions()
	env.evalPrevAndNextVersion()

	tagVersion := fmt.Sprintf("v%v", env.nextVersion)

	branchName := env.getCmdOutputOneLine("get git branch", "git", "rev-parse", "--abbrev-ref", "HEAD")

	if val, found := os.LookupEnv("TRAVIS_PULL_REQUEST_BRANCH"); found && val != "" {
		branchName = val
	} else if val, found := os.LookupEnv("TRAVIS_BRANCH"); found && val != "" {
		branchName = val
	}

	buildInfo := &BuildInfo{
		PackageName: args[1],
		Version:     tagVersion,
		Revision:    env.getCmdOutputOneLine("get git SHA", "git", "rev-parse", "--short=12", "HEAD"),
		Branch:      branchName,
		BuildUser:   getUsername(cmd),
		BuildDate:   time.Now().Format("2006-01-02 15:04:05"),
	}

	compiledTemplate, err := template.New("buildInfo").Parse(buildInfoTemplate)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "failure compiling build info template %+v\n", err)
		os.Exit(-1)
	}

	file, err := os.Create(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "failure opening build info output file %v. err: %+v\n", args[0], err)
		os.Exit(-1)
	}

	err = compiledTemplate.Execute(file, buildInfo)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "failure executing build template to output file %v. err: %+v\n", args[0], err)
		os.Exit(-1)
	}

	env.runGitCommand("add build info file to git", "add", args[0])
	env.runGitCommand("commit build info file", "commit", "-m", fmt.Sprintf("Release %v", tagVersion))
}

func getBaseVersion(cmd *cobra.Command) *version.Version {
	if baseVersionString == "" {
		if baseVersionFile == "" {
			baseVersionFile = DefaultVersionFile
		}
		contents, err := ioutil.ReadFile(baseVersionFile)
		if err != nil {
			currdir, _ := os.Getwd()
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to load base version information from '%v'. current dir: '%v'\n", baseVersionFile, currdir)

			contents, err = ioutil.ReadFile("./common/version/VERSION")
			if err != nil {
				currdir, _ = os.Getwd()
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to load base version information from '%v'. current dir: '%v'\n", baseVersionFile, currdir)
				os.Exit(-1)
			}
		}
		baseVersionString = string(contents)
		baseVersionString = strings.TrimSpace(baseVersionString)
	}
	baseVersion, err := version.NewVersion(baseVersionString)
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Invalid base version %v\n", baseVersionString)
		os.Exit(-1)
	}
	return baseVersion
}

func getUsername(cmd *cobra.Command) string {
	currUser, err := user.Current()
	if err != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to get current user %+v\n", err)
		return "unknown"
	}
	return currUser.Name
}

func showBuildInfo(*cobra.Command, []string) {
	fmt.Printf("ziti-cmd version: %v, revision: %v, branch: %v, build-by: %v, built-on: %v\n", Version, Revision, Branch, BuildUser, BuildDate)
}

func configureGit(cmd *cobra.Command, args []string) {
	env := newRunEnv(cmd, args)
	env.setupGitEnv(gitUsername, gitEmail)

	if val, found := os.LookupEnv(sshKeyEnv); found && val != "" {
		sshKey, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to decode ssh key. err: %v\n", err)
			os.Exit(-1)
		}
		if err = ioutil.WriteFile(sshKeyFile, sshKey, 0600); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to write ssh key file %v. err: %v\n", sshKeyFile, err)
			os.Exit(-1)
		}
	} else {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "unable to read ssh key from env var %v. Found? %v\n", sshKeyEnv, found)
		os.Exit(-1)
	}
}
