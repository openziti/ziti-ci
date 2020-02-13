package main

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"net/http"
	"net/url"
	"os"
)

type triggerTravisBuidlCmd struct {
	baseCommand

	travisToken string
}

func (cmd *triggerTravisBuidlCmd) execute() {
	cmd.evalCurrentAndNextVersion()

	if cmd.travisToken == "" {
		found := false
		cmd.travisToken, found = os.LookupEnv("travis_token")
		if !found {
			cmd.failf("no travis token provided. Unable to trigger builds\n")
		}
	}

	bodyTemplate := `
		{
			"request": {
  	  			"branch":"%v",
      			"message": "ziti-ci:update-dependency %v"
  			}
		}`

	branch := cmd.args[1]
	module := fmt.Sprintf("github.com/%v@%v", os.Getenv("TRAVIS_REPO_SLUG"), cmd.currentVersion.String())
	body := fmt.Sprintf(bodyTemplate, branch, module)

	client := resty.New()

	targetRepo := url.QueryEscape(cmd.args[0])
	targetUrl := fmt.Sprintf("https://api.travis-ci.org/repo/%v/requests", targetRepo)

	resp, err := client.R().
		EnableTrace().
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Travis-API-Version", "3").
		SetHeader("Authorization", fmt.Sprintf("token %v", cmd.travisToken)).
		SetBody(body).
		Post(targetUrl)

	if err != nil {
		cmd.failf("Error triggering build s\n")
		panic(err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusAccepted {
		cmd.logJson(resp.Body())
		cmd.failf("Error triggering build. REST call returned %v", resp.StatusCode())
	}
}

func newTriggerTravisBuildCmd(root *rootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "trigger-travis-build <target-repo> <target-branch>",
		Short: "Trigger a Travis CI build",
		Args:  cobra.ExactArgs(2),
	}

	result := &triggerTravisBuidlCmd{
		baseCommand: baseCommand{
			rootCommand: root,
			cmd:         cobraCmd,
		},
	}

	cobraCmd.PersistentFlags().StringVar(&result.travisToken, "token", "", "Travis token to use to trigger the build")

	return finalize(result)
}
