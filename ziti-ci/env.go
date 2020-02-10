package main

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"sort"
	"strings"
)

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

func (env *runEnv) getCmdOutputOneLine(description string, cmd string, params ...string) string {
	output := env.runCommandWithOutput(description, cmd, params...)
	if len(output) != 1 {
		_, _ = fmt.Fprintf(env.cmd.OutOrStderr(), "expected 1 line return from %v: %v %v, but got %v\n", description, cmd, strings.Join(params, " "), len(output))
	}
	return output[0]
}

func (env *runEnv) getGoEnv() map[string]string {
	lines := env.runCommandWithOutput("get go environment", "go", "env", "-json")
	result := map[string]string{}
	err := json.Unmarshal([]byte(strings.Join(lines, "\n")), &result)
	if err != nil {
		_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "error unmarshalling go env json: %v\n", err)
		os.Exit(-1)
	}
	return result
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
