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
	"bufio"
	"encoding/base64"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type configureGitCmd struct {
	BaseCommand

	gitUsername string
	gitEmail    string

	sshKeyEnv  string
	sshKeyFile string
}

func (cmd *configureGitCmd) Execute() {
	if val, found := os.LookupEnv("GITHUB_REPOSITORY_OWNER"); found && (val != "openziti" && val != "netfoundry") {
		cmd.Warnf("Running in non-openziti context. Not attempting to configure git.\n")
		return
	}

	cmd.Infof("running in openziti context, configuring git\n")
	if val, found := os.LookupEnv(cmd.sshKeyEnv); found && val != "" {
		sshKey, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			cmd.Failf("unable to decode ssh key. err: %v\n", err)
		}
		if err = ioutil.WriteFile(cmd.sshKeyFile, sshKey, 0600); err != nil {
			cmd.Failf("unable to write ssh key file %v. err: %v\n", cmd.sshKeyFile, err)
		}
	} else {
		cmd.Failf("unable to read ssh key from env var %v. Found? %v\n", cmd.sshKeyEnv, found)
	}

	kfAbs, err := filepath.Abs(cmd.sshKeyFile)
	if err != nil {
		cmd.Failf("unable to read path for sshKeyFile? %v\n", cmd.sshKeyFile)
	}

	keyDir := path.Dir(kfAbs)

	ignoreExists := false
	if file, err := os.Open(keyDir + string(os.PathSeparator) + ".gitignore"); err == nil {
		// if err, file probably isn't there etc. just ignore this particular error
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), cmd.sshKeyFile) {
				ignoreExists = true
			}
		}
		file.Close()
	} else {
		cmd.Infof("unable to scan .gitignore: %v\n", err)
	}

	if !ignoreExists {
		cmd.Infof("adding " + cmd.sshKeyFile + " to .gitignore\n")
		//add the deploy key to .gitignore... next to whereever the sshkey goes...
		f, err := os.OpenFile(keyDir+string(os.PathSeparator)+".gitignore",
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			cmd.Failf("could not write to .gitignore (%v)\n", err)
		}
		defer f.Close()
		if _, err := f.WriteString("\n" + cmd.sshKeyFile + "\n"); err != nil {
			cmd.Failf("error writing to .gitignore (%v)\n", err)
		}
	} else {
		cmd.Infof(".gitignore file already contains entry for %v\n", cmd.sshKeyFile)
	}

	if val, found := os.LookupEnv(DefaultGpgKeyEnvVar); found && val != "" {
		if val, found := os.LookupEnv(DefaultGpgKeyIdEnvVar); found && val != "" {
			cmd.RunGitCommand("set gpg key id", "config", "user.signingkey", val)
		} else {
			cmd.Failf("unable to read gpg key from env var %v. Found? %v\n", DefaultGpgKeyIdEnvVar, found)
		}

		if err = os.WriteFile("gpg.key", []byte(val), 0600); err != nil {
			cmd.Failf("unable to write gpg key file [%v]. err: (%v)\n", cmd.sshKeyFile, err)
		}
		cmd.runCommand("import gpg key", "gpg", "--import", "gpg.key")
		if err = os.Remove("gpg.key"); err != nil {
			cmd.Failf("unable to delete gpg.key (%v)\n", err)
		}
		cmd.RunGitCommand("require gpg signed commit", "config", "commit.gpgsign", "true")
		cmd.RunGitCommand("require gpg signed tags", "config", "tag.gpgSign", "true")

	} else {
		cmd.Warnf("unable to read gpg key from env var %v. Found? %v\n", DefaultGpgKeyEnvVar, found)
	}

	cmd.RunGitCommand("set git username", "config", "user.name", cmd.gitUsername)
	cmd.RunGitCommand("set git password", "config", "user.email", cmd.gitEmail)
	cmd.RunGitCommand("set ssh config", "config", "core.sshCommand", fmt.Sprintf("ssh -i %v", cmd.sshKeyFile))

	repo := ""
	if travisRepoSlug, ok := os.LookupEnv("TRAVIS_REPO_SLUG"); ok {
		repo = travisRepoSlug
	}

	if githubRepo, ok := os.LookupEnv("GITHUB_REPOSITORY"); ok {
		repo = githubRepo
	}

	// Ensure we're in ssh mode
	if repo != "" {
		url := fmt.Sprintf("git@github.com:%v.git", repo)
		cmd.RunGitCommand("set remote to ssh", "remote", "set-url", "origin", url)
	}
}

func newConfigureGitCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "configure-git",
		Short: "Configure git",
		Args:  cobra.ExactArgs(0),
	}

	result := &configureGitCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.PersistentFlags().StringVar(&result.gitUsername, "git-username", DefaultGitUsername, "override the default git username")
	cobraCmd.PersistentFlags().StringVar(&result.gitEmail, "git-email", DefaultGitEmail, "override the default git email")
	cobraCmd.PersistentFlags().StringVar(&result.sshKeyEnv, "ssh-key-env-var", DefaultSshKeyEnvVar, "set ssh key environment variable name")
	cobraCmd.PersistentFlags().StringVar(&result.sshKeyFile, "ssh-key-file", DefaultSshKeyFile, "set ssh key file name")

	return Finalize(result)
}
