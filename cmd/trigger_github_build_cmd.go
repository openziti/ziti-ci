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
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
)

type triggerGithubBuidlCmd struct {
	BaseCommand
	githubToken string
}

func (cmd *triggerGithubBuidlCmd) Execute() {
	cmd.EvalCurrentAndNextVersion()

	if cmd.githubToken == "" {
		found := false
		cmd.githubToken, found = os.LookupEnv("GITHUB_TOKEN")
		if !found {
			cmd.Failf("no github token provided. Unable to trigger builds\n")
		}
	}

	bodyTemplate := `
		{
			"request": {
  	  			"branch":"%v",
      			"message": "ziti-ci:update-dependency %v",
 				"config": {
   					"merge_mode": "deep_merge_append",
   					"env": {
     					"global": ["UPDATED_DEPENDENCY=%v"]
                    }
				}
  			}
		}`

	branch := cmd.Args[1]
	module := fmt.Sprintf("%v@v%v", cmd.getModule(), cmd.CurrentVersion.String())
	body := fmt.Sprintf(bodyTemplate, branch, module, module)

	client := resty.New()

	targetRepo := url.QueryEscape(cmd.Args[0])
	targetUrl := fmt.Sprintf("https://api.travis-ci.org/repo/%v/requests", targetRepo)

	resp, err := client.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Github-API-Version", "3").
		SetHeader("Authorization", fmt.Sprintf("token %v", cmd.githubToken)).
		SetBody(body).
		Post(targetUrl)

	if err != nil {
		cmd.Failf("Error triggering build s\n")
		panic(err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusAccepted {
		cmd.logJson(resp.Body())
		cmd.Failf("Error triggering build. REST call returned %v", resp.StatusCode())
	}

	cmd.Infof("successfully triggered build of %v to update to %v\n", cmd.Args[0], module)
}

func newTriggerGithubBuildCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "trigger-travis-build <target-repo> <target-branch>",
		Short: "Trigger a Github CI build",
		Args:  cobra.ExactArgs(2),
	}

	result := &triggerGithubBuidlCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.PersistentFlags().StringVar(&result.githubToken, "token", "", "Github token to use to trigger the build")

	return Finalize(result)
}
