package main

import (
	"encoding/base64"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
)

const (
	DefaultSshKeyEnvVar = "gh_ci_key"
	DefaultSshKeyFile   = "github_deploy_key"
)

type configureGitCmd struct {
	baseCommand

	gitUsername string
	gitEmail    string

	sshKeyEnv  string
	sshKeyFile string
}

func (cmd *configureGitCmd) execute() {
	if val, found := os.LookupEnv(cmd.sshKeyEnv); found && val != "" {
		sshKey, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			cmd.failf("unable to decode ssh key. err: %v\n", err)
		}
		if err = ioutil.WriteFile(cmd.sshKeyFile, sshKey, 0600); err != nil {
			cmd.failf("unable to write ssh key file %v. err: %v\n", cmd.sshKeyFile, err)
		}
	} else {
		cmd.failf("unable to read ssh key from env var %v. Found? %v\n", cmd.sshKeyEnv, found)
	}

	cmd.runGitCommand("set git username", "config", "user.name", cmd.gitUsername)
	cmd.runGitCommand("set git password", "config", "user.email", cmd.gitEmail)
	cmd.runGitCommand("set ssh config", "config", "core.sshCommand", fmt.Sprintf("ssh -i %v", cmd.sshKeyFile))
}

func newConfigureGitCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "configure-git",
		Short: "Configure git",
		Args:  cobra.ExactArgs(0),
	}

	result := &configureGitCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	cobraCmd.PersistentFlags().StringVar(&result.gitUsername, "git-username", "ziti-ci", "override the default git username")
	cobraCmd.PersistentFlags().StringVar(&result.gitEmail, "git-email", "ziti-ci@netfoundry.io", "override the default git email")
	cobraCmd.PersistentFlags().StringVar(&result.sshKeyEnv, "ssh-key-env-var", DefaultSshKeyEnvVar, "set ssh key environment variable name")
	cobraCmd.PersistentFlags().StringVar(&result.sshKeyFile, "ssh-key-file", DefaultSshKeyFile, "set ssh key file name")

	return finalize(result)
}
