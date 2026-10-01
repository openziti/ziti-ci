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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	method string
	uri    string
	auth   string
	accept string
	body   map[string]string
}

// newTestGithubRestApi returns a client for openziti/test pointed at a server that answers the
// requests close-backport-issues makes, and the requests the server has received.
func newTestGithubRestApi(t *testing.T) (*githubRestApi, *[]recordedRequest) {
	var requests []recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedRequest{method: r.Method, uri: r.URL.RequestURI(), auth: r.Header.Get("Authorization"), accept: r.Header.Get("Accept")}
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			require.NoError(t, json.Unmarshal(data, &rec.body))
		}
		requests = append(requests, rec)

		switch rec.method + " " + rec.uri {
		case "GET /repos/openziti/test/pulls/5":
			_, _ = io.WriteString(w, `{"merged":true,"merge_commit_sha":"abc","body":null,"base":{"ref":"release-v2.0.x"}}`)
		case "GET /repos/openziti/test/pulls/5/commits?per_page=100&page=1":
			commits := make([]string, 100)
			for i := range commits {
				commits[i] = fmt.Sprintf(`{"commit":{"message":"commit %d"}}`, i)
			}
			_, _ = io.WriteString(w, "["+strings.Join(commits, ",")+"]")
		case "GET /repos/openziti/test/pulls/5/commits?per_page=100&page=2":
			_, _ = io.WriteString(w, `[{"commit":{"message":"last"}}]`)
		case "GET /repos/openziti/test/issues/7":
			_, _ = io.WriteString(w, `{"title":"[Backport-2.0] An issue","state":"open"}`)
		case "GET /repos/openziti/test/issues/8":
			_, _ = io.WriteString(w, `{"title":"A pull request","state":"open","pull_request":{"url":"x"}}`)
		case "GET /repos/openziti/test/contents/CHANGELOG.md?ref=abc":
			_, _ = io.WriteString(w, "# Release 2.0.8\n")
		case "GET /repos/openziti/test/git/ref/tags/v2.0.7":
			_, _ = io.WriteString(w, `{"ref":"refs/tags/v2.0.7"}`)
		case "POST /repos/openziti/test/issues/7/comments":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{}`)
		case "PATCH /repos/openziti/test/issues/7":
			_, _ = io.WriteString(w, `{}`)
		case "GET /repos/openziti/test/issues/9":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"boom"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"message":"Not Found"}`)
		}
	}))
	t.Cleanup(server.Close)

	api := newGithubRestApi("openziti/test", "tok")
	api.client.SetBaseURL(server.URL)
	return api, &requests
}

func TestGithubRestApiReads(t *testing.T) {
	req := require.New(t)
	api, requests := newTestGithubRestApi(t)

	pr, err := api.GetPullRequest(5)
	req.NoError(err)
	req.True(pr.Merged)
	req.Equal("abc", pr.MergeCommitSha)
	req.Equal("", pr.Body)
	req.Equal("release-v2.0.x", pr.Base.Ref)

	messages, err := api.GetPullRequestCommitMessages(5)
	req.NoError(err)
	req.Len(messages, 101)
	req.Equal("last", messages[100])

	issue, err := api.GetIssue(7)
	req.NoError(err)
	req.Equal("[Backport-2.0] An issue", issue.Title)
	req.False(issue.IsPullRequest())

	issue, err = api.GetIssue(8)
	req.NoError(err)
	req.True(issue.IsPullRequest())

	_, err = api.GetIssue(9)
	req.ErrorContains(err, "returned 500")

	_, err = api.GetIssue(10)
	req.ErrorIs(err, errNotFound)

	contents, found, err := api.GetFile("CHANGELOG.md", "abc")
	req.NoError(err)
	req.True(found)
	req.Equal("# Release 2.0.8\n", contents)
	req.Equal("application/vnd.github.raw", (*requests)[len(*requests)-1].accept)

	_, found, err = api.GetFile("MISSING.md", "abc")
	req.NoError(err)
	req.False(found)

	exists, err := api.TagExists("v2.0.7")
	req.NoError(err)
	req.True(exists)

	exists, err = api.TagExists("v2.0.8")
	req.NoError(err)
	req.False(exists)

	for _, r := range *requests {
		req.Equal("Bearer tok", r.auth, r.uri)
	}
}

func TestGithubRestApiCommentAndClose(t *testing.T) {
	req := require.New(t)
	api, requests := newTestGithubRestApi(t)

	req.NoError(api.CommentAndClose(7, "Fixed on `release-v2.0.x` by #5."))

	req.Len(*requests, 2)
	req.Equal("POST", (*requests)[0].method)
	req.Equal("/repos/openziti/test/issues/7/comments", (*requests)[0].uri)
	req.Equal(map[string]string{"body": "Fixed on `release-v2.0.x` by #5."}, (*requests)[0].body)
	req.Equal("PATCH", (*requests)[1].method)
	req.Equal("/repos/openziti/test/issues/7", (*requests)[1].uri)
	req.Equal(map[string]string{"state": "closed", "state_reason": "completed"}, (*requests)[1].body)
}

func TestGithubRestApiCommentFailureLeavesIssueOpen(t *testing.T) {
	req := require.New(t)
	api, requests := newTestGithubRestApi(t)

	req.ErrorIs(api.CommentAndClose(12, "body"), errNotFound)
	req.Len(*requests, 1, "the issue is not closed when the comment fails")
}
