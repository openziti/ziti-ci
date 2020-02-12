package main

import (
	"fmt"
	"os"
)

func (env *runEnv) updateGoDependency() {
	env.runGitCommand("Ensure origin/master is up to date", "fetch")
	env.runGitCommand("Sync with master", "merge", "-ff-only", "origin/master")
	dep := env.getUpdatedDep()
	env.runCommand("Update dependency", "go", "get", dep)
	diffOutput := env.runCommandWithOutput("check if there's a change", "git", "diff", "--name-only", "go.mod")
	if len(diffOutput) != 1 || diffOutput[0] != "go.mod" {
		_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "requested dependency did not result in change\n")
		os.Exit(-1)
	}
	_, _ = fmt.Fprintf(env.cmd.OutOrStdout(), "attempting to update to %v\n", dep)

	env.runCommand("Tidy go.sum", "go", "mod", "tidy")
	env.runGitCommand("Add go mod changes", "add", "go.mod", "go.sum")
	env.runGitCommand("Commit go.mod changes", "commit", "-m", fmt.Sprintf("Updating dependency %v", dep))
}

func (env *runEnv) completeGoDependencyUpdate() {
	currentCommit := env.getCmdOutputOneLine("get git SHA", "git", "rev-parse", "--short=12", "HEAD")
	env.runGitCommand("Checkout master", "co", "master")
	env.runGitCommand("Merge in update branch", "merge", "-ff-only", currentCommit)
	env.runGitCommand("Push to remote", "push")
}

func (env *runEnv) getUpdatedDep() string {
	newDep := ""
	if len(env.args) > 0 {
		newDep = env.args[0]
	}
	if newDep == "" {
		newDep = os.Getenv("TRAVIS_COMMIT_MESSAGE")
	}

	if newDep == "" {
		_, _ = fmt.Fprintf(env.cmd.ErrOrStderr(), "no updated dependency provided\n")
		os.Exit(-1)
	}

	return newDep
}
