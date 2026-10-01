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
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestBackportPrefix(t *testing.T) {
	tests := []struct {
		branch string
		prefix string
		ok     bool
	}{
		{"release-v2.0.x", "[Backport-2.0]", true},
		{"release-v1.6.x", "[Backport-1.6]", true},
		{"release-v0.34.x", "[Backport-0.34]", true},
		{"release-v4.x", "[Backport-4.x]", true},
		{"main", "", false},
		{"release-v1.2.4-x", "", false},
		{"release-0.5.x", "", false},
		{"release-v2.0.x-hotfix", "", false},
	}
	for _, test := range tests {
		t.Run(test.branch, func(t *testing.T) {
			prefix, ok := backportPrefix(test.branch)
			require.Equal(t, test.ok, ok)
			require.Equal(t, test.prefix, prefix)
		})
	}
}

type fakeGithubApi struct {
	prs         map[int]*githubPullRequest
	commits     map[int][]string
	issues      map[int]*githubIssue
	issueErrors map[int]error
	files       map[string]string
	tags        map[string]bool
	comments    map[int][]string
	closed      []int
	issueReads  []int
}

func newFakeGithubApi() *fakeGithubApi {
	return &fakeGithubApi{
		prs:         map[int]*githubPullRequest{},
		commits:     map[int][]string{},
		issues:      map[int]*githubIssue{},
		issueErrors: map[int]error{},
		files:       map[string]string{},
		tags:        map[string]bool{},
		comments:    map[int][]string{},
	}
}

func (self *fakeGithubApi) addPr(number int, base string, body string, commits ...string) {
	pr := &githubPullRequest{Merged: true, MergeCommitSha: "abcdef1234567890", Body: body}
	pr.Base.Ref = base
	self.prs[number] = pr
	self.commits[number] = commits
}

func (self *fakeGithubApi) addIssue(number int, title string, state string) {
	self.issues[number] = &githubIssue{Title: title, State: state}
}

func (self *fakeGithubApi) GetPullRequest(number int) (*githubPullRequest, error) {
	if pr, found := self.prs[number]; found {
		return pr, nil
	}
	return nil, errNotFound
}

func (self *fakeGithubApi) GetPullRequestCommitMessages(number int) ([]string, error) {
	return self.commits[number], nil
}

func (self *fakeGithubApi) GetIssue(number int) (*githubIssue, error) {
	self.issueReads = append(self.issueReads, number)
	if err := self.issueErrors[number]; err != nil {
		return nil, err
	}
	if issue, found := self.issues[number]; found {
		return issue, nil
	}
	return nil, errNotFound
}

func (self *fakeGithubApi) GetFile(path string, ref string) (string, bool, error) {
	contents, found := self.files[path+"@"+ref]
	return contents, found, nil
}

func (self *fakeGithubApi) TagExists(tag string) (bool, error) {
	return self.tags[tag], nil
}

func (self *fakeGithubApi) Comment(number int, body string) error {
	self.comments[number] = append(self.comments[number], body)
	return nil
}

func (self *fakeGithubApi) CommentAndClose(number int, body string) error {
	self.closed = append(self.closed, number)
	return self.Comment(number, body)
}

func newTestCloseBackportIssuesCmd(api githubApi, dryRun bool, commentOnSkipped bool) (*closeBackportIssuesCmd, *bytes.Buffer) {
	out := &bytes.Buffer{}
	cobraCmd := &cobra.Command{}
	cobraCmd.SetOut(out)
	return &closeBackportIssuesCmd{
		BaseCommand: BaseCommand{
			RootCommand: &RootCommand{dryRun: dryRun},
			Cmd:         cobraCmd,
		},
		repo:             "openziti/ziti",
		commentOnSkipped: commentOnSkipped,
		api:              api,
	}, out
}

// newReleasePrApi returns a fake holding PR #100, merged to release-v2.0.x, whose description and
// commits reference one issue of each kind the command distinguishes.
func newReleasePrApi() *fakeGithubApi {
	api := newFakeGithubApi()
	api.addPr(100, "release-v2.0.x", "Fixes #10 (backport of #1 to `release-v2.0.x`).",
		"Retry the link announcement. Fixes #10",
		"Close a dialed link the registry cannot account for. closes openziti/ziti#11",
		"Fix the panic. Fixes #12, fixes #13, fixes #14, fixes openziti/sdk-golang#15",
		"Fix the original. Fixes #16")
	api.addIssue(10, "[Backport-2.0] Router gives up announcing its links", "open")
	api.addIssue(11, "[Backport-2.0] A dialed link the router has no state for", "open")
	api.addIssue(12, "[Backport-2.0] Already done", "closed")
	api.issues[13] = &githubIssue{Title: "Some pull request", State: "open", PullRequest: json.RawMessage(`{"url":"x"}`)}
	api.addIssue(14, "[Backport-1.6] The same fix for another branch", "open")
	api.addIssue(16, "Router gives up announcing its links", "open")
	api.files["CHANGELOG.md@abcdef1234567890"] = "# Release 2.0.8\n\n## What's New\n\n# Release 2.0.7\n"
	api.tags["v2.0.7"] = true
	return api
}

func TestCloseBackportIssuesClosesOpenBackportIssues(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	cmd, _ := newTestCloseBackportIssuesCmd(api, false, true)

	req.NoError(cmd.processPullRequest(100))

	req.Equal([]int{10, 11}, api.closed)
	expected := "Fixed on `release-v2.0.x` by #100 (commit abcdef123), to be released in 2.0.8."
	req.Equal([]string{expected}, api.comments[10])
	req.Equal([]string{expected}, api.comments[11])

	req.Empty(api.comments[12], "an already closed issue is left alone")
	req.Empty(api.comments[13], "a pull request is not closed")
	req.NotContains(api.issueReads, 15, "a reference to another repository is not looked up")

	req.Len(api.comments[100], 1)
	req.Contains(api.comments[100][0], "- #14 [Backport-1.6] The same fix for another branch")
	req.Contains(api.comments[100][0], "- #16 Router gives up announcing its links")
	req.Contains(api.comments[100][0], "reference that branch's `[Backport-2.0]` tracking issue")
	req.NotContains(api.comments[100][0], "#13")
}

func TestCloseBackportIssuesReadsEachIssueOnce(t *testing.T) {
	api := newReleasePrApi()
	cmd, _ := newTestCloseBackportIssuesCmd(api, true, false)

	require.NoError(t, cmd.processPullRequest(100))
	require.Equal(t, []int{10, 11, 12, 13, 14, 16}, api.issueReads)
}

func TestCloseBackportIssuesDryRunChangesNothing(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	cmd, out := newTestCloseBackportIssuesCmd(api, true, true)

	req.NoError(cmd.processPullRequest(100))

	req.Empty(api.closed)
	req.Empty(api.comments)
	req.Contains(out.String(), "closing #10")
	req.Contains(out.String(), "commenting on #100")
}

func TestCloseBackportIssuesCommentsOnSkippedOnlyWhenAsked(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	cmd, out := newTestCloseBackportIssuesCmd(api, false, false)

	req.NoError(cmd.processPullRequest(100))

	req.Equal([]int{10, 11}, api.closed)
	req.Empty(api.comments[100])
	req.Contains(out.String(), "#16 (Router gives up announcing its links), which is not a [Backport-2.0] issue")
}

func TestCloseBackportIssuesOmitsReleasedVersion(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	api.tags["v2.0.8"] = true
	cmd, _ := newTestCloseBackportIssuesCmd(api, false, false)

	req.NoError(cmd.processPullRequest(100))
	req.Equal([]string{"Fixed on `release-v2.0.x` by #100 (commit abcdef123)."}, api.comments[10])
}

func TestCloseBackportIssuesOmitsVersionWithoutChangelog(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	delete(api.files, "CHANGELOG.md@abcdef1234567890")
	cmd, _ := newTestCloseBackportIssuesCmd(api, false, false)

	req.NoError(cmd.processPullRequest(100))
	req.Equal([]string{"Fixed on `release-v2.0.x` by #100 (commit abcdef123)."}, api.comments[10])
}

func TestCloseBackportIssuesUsesMajorLinePrefix(t *testing.T) {
	req := require.New(t)
	api := newFakeGithubApi()
	api.addPr(300, "release-v4.x", "Fixes #302")
	api.addIssue(302, "[Backport-4.x] ReplyFor header panics", "open")
	cmd, _ := newTestCloseBackportIssuesCmd(api, false, false)
	cmd.repo = "openziti/channel"

	req.NoError(cmd.processPullRequest(300))
	req.Equal([]int{302}, api.closed)
}

func TestCloseBackportIssuesSkipsUnmergedAndNonReleasePullRequests(t *testing.T) {
	req := require.New(t)
	api := newFakeGithubApi()
	api.addPr(200, "main", "Fixes #20")
	api.addPr(201, "release-v2.0.x", "Fixes #21")
	api.prs[201].Merged = false
	api.addIssue(20, "[Backport-2.0] On main", "open")
	api.addIssue(21, "[Backport-2.0] Not merged", "open")
	cmd, out := newTestCloseBackportIssuesCmd(api, false, true)

	req.NoError(cmd.processPullRequest(200))
	req.NoError(cmd.processPullRequest(201))

	req.Empty(api.issueReads)
	req.Empty(api.closed)
	req.Empty(api.comments)
	req.Contains(out.String(), "#200 targets main, which is not a release branch")
	req.Contains(out.String(), "#201 is not merged")
}

func TestCloseBackportIssuesKeepsGoingAfterAnUnreadableIssue(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	api.issueErrors[10] = errors.New("rate limited")
	cmd, _ := newTestCloseBackportIssuesCmd(api, false, false)

	err := cmd.processPullRequest(100)

	req.ErrorContains(err, "rate limited")
	req.Equal([]int{11}, api.closed)
}

func TestCloseBackportIssuesExecuteProcessesEachArgument(t *testing.T) {
	req := require.New(t)
	api := newReleasePrApi()
	api.addPr(101, "release-v2.0.x", "Fixes #17")
	api.addIssue(17, "[Backport-2.0] Another", "open")
	cmd, _ := newTestCloseBackportIssuesCmd(api, false, false)
	cmd.Args = []string{"#100", "101"}

	req.NoError(cmd.Execute())
	req.Equal([]int{10, 11, 17}, api.closed)

	cmd.Args = []string{"one"}
	req.ErrorContains(cmd.Execute(), `invalid pull request number "one"`)

	cmd.Args = []string{"999"}
	req.ErrorIs(cmd.Execute(), errNotFound)
}
