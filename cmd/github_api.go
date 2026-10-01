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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
)

type githubPullRequest struct {
	Merged         bool   `json:"merged"`
	MergeCommitSha string `json:"merge_commit_sha"`
	Body           string `json:"body"`
	Base           struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

type githubIssue struct {
	Title       string          `json:"title"`
	State       string          `json:"state"`
	PullRequest json.RawMessage `json:"pull_request"`
}

// IsPullRequest reports whether the issue is a pull request, which the issues API also returns.
func (self *githubIssue) IsPullRequest() bool {
	return len(self.PullRequest) > 0 && string(self.PullRequest) != "null"
}

// githubApi is the part of the GitHub REST API that close-backport-issues uses, scoped to one
// repository.
type githubApi interface {
	GetPullRequest(number int) (*githubPullRequest, error)

	// GetPullRequestCommitMessages returns the messages of a pull request's commits, of which GitHub
	// lists at most 250.
	GetPullRequestCommitMessages(number int) ([]string, error)

	// GetIssue returns an issue or pull request by number.
	GetIssue(number int) (*githubIssue, error)

	// GetFile returns the contents of path at ref, or false if there is no such file.
	GetFile(path string, ref string) (string, bool, error)

	TagExists(tag string) (bool, error)

	// Comment adds a comment to an issue or pull request.
	Comment(number int, body string) error

	// CommentAndClose adds a comment to an issue and closes it as completed.
	CommentAndClose(number int, body string) error
}

var errNotFound = errors.New("not found")

type githubRestApi struct {
	client *resty.Client
	repo   string
}

func newGithubRestApi(repo string, token string) *githubRestApi {
	return &githubRestApi{
		client: resty.New().
			SetBaseURL("https://api.github.com").
			SetHeader("Accept", "application/vnd.github+json").
			SetHeader("X-GitHub-Api-Version", "2022-11-28").
			SetAuthToken(token),
		repo: repo,
	}
}

// do sends a request for a path under the repository, decoding a successful response into result
// when it's non-nil. A 404 is returned wrapping errNotFound.
func (self *githubRestApi) do(req *resty.Request, method string, path string, body any, result any) (*resty.Response, error) {
	if body != nil {
		req.SetBody(body)
	}
	path = "/repos/" + self.repo + path
	resp, err := req.Execute(method, path)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("%s %s: %w", method, path, errNotFound)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("%s %s returned %d: %s", method, path, resp.StatusCode(), strings.TrimSpace(resp.String()))
	}
	if result != nil {
		if err = json.Unmarshal(resp.Body(), result); err != nil {
			return nil, fmt.Errorf("%s %s: decoding response: %w", method, path, err)
		}
	}
	return resp, nil
}

func (self *githubRestApi) GetPullRequest(number int) (*githubPullRequest, error) {
	result := &githubPullRequest{}
	if _, err := self.do(self.client.R(), http.MethodGet, fmt.Sprintf("/pulls/%d", number), nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (self *githubRestApi) GetPullRequestCommitMessages(number int) ([]string, error) {
	const pageSize = 100
	var messages []string
	for page := 1; ; page++ {
		var commits []struct {
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		}
		path := fmt.Sprintf("/pulls/%d/commits?per_page=%d&page=%d", number, pageSize, page)
		if _, err := self.do(self.client.R(), http.MethodGet, path, nil, &commits); err != nil {
			return nil, err
		}
		for _, c := range commits {
			messages = append(messages, c.Commit.Message)
		}
		if len(commits) < pageSize {
			return messages, nil
		}
	}
}

func (self *githubRestApi) GetIssue(number int) (*githubIssue, error) {
	result := &githubIssue{}
	if _, err := self.do(self.client.R(), http.MethodGet, fmt.Sprintf("/issues/%d", number), nil, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (self *githubRestApi) GetFile(path string, ref string) (string, bool, error) {
	req := self.client.R().SetHeader("Accept", "application/vnd.github.raw")
	resp, err := self.do(req, http.MethodGet, "/contents/"+path+"?ref="+url.QueryEscape(ref), nil, nil)
	if errors.Is(err, errNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(resp.Body()), true, nil
}

func (self *githubRestApi) TagExists(tag string) (bool, error) {
	_, err := self.do(self.client.R(), http.MethodGet, "/git/ref/tags/"+tag, nil, nil)
	if errors.Is(err, errNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (self *githubRestApi) Comment(number int, body string) error {
	_, err := self.do(self.client.R(), http.MethodPost, fmt.Sprintf("/issues/%d/comments", number),
		map[string]string{"body": body}, nil)
	return err
}

func (self *githubRestApi) CommentAndClose(number int, body string) error {
	if err := self.Comment(number, body); err != nil {
		return err
	}
	_, err := self.do(self.client.R(), http.MethodPatch, fmt.Sprintf("/issues/%d", number),
		map[string]string{"state": "closed", "state_reason": "completed"}, nil)
	return err
}
