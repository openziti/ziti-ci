package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

const (
	DefaultVersionFile = "./version"
)

type tagCmd struct {
	baseCommand
}

func (cmd *tagCmd) execute() {
	cmd.evalCurrentAndNextVersion()

	headTags := cmd.getVersionList("tag", "--points-at", "HEAD")
	if len(headTags) > 0 {
		cmd.errorf("head already tagged with %+v:\n", headTags)
		os.Exit(0)
	}

	cmd.infof("previous version: %v, next version: %v\n", cmd.currentVersion, cmd.nextVersion)

	if cmd.isGoLang() {
		nextMajorVersion := cmd.nextVersion.Segments()[0]
		if nextMajorVersion > 1 {
			moduleName := cmd.getModule()
			if !strings.HasSuffix(moduleName, fmt.Sprintf("/v%v", nextMajorVersion)) {
				cmd.failf("error: module version doesn't match next version: %v\n", nextMajorVersion)
			}
		}
	}

	tagVersion := fmt.Sprintf("%v", cmd.nextVersion)
	if cmd.isGoLang() {
		tagVersion = "v" + tagVersion
	}
	tagParms := []string{"tag", "-a", tagVersion, "-m", fmt.Sprintf("Release %v", tagVersion)}
	cmd.runGitCommand("create tag", tagParms...)

	// Ensure we're in ssh mode
	if repoSlug, ok := os.LookupEnv("TRAVIS_REPO_SLUG"); ok {
		url := fmt.Sprintf("git@github.com:%v.git", repoSlug)
		cmd.runGitCommand("set remote to ssh", "remote", "set-url", "origin", url)
	}

	cmd.runGitCommand("push tag to repo", "push", "origin", tagVersion)
}

func newTagCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "tag",
		Short: "Tag and push command",
		Args:  cobra.ExactArgs(0),
	}

	result := &tagCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	return finalize(result)
}
