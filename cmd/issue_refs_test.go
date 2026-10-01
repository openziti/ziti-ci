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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClosingIssueRefs(t *testing.T) {
	bare := func(n int) IssueRef { return IssueRef{Number: n} }
	in := func(repo string, n int) IssueRef { return IssueRef{Repo: repo, Number: n} }

	tests := []struct {
		name string
		text string
		want []IssueRef
	}{
		{"every keyword and tense", "close #1, closes #2, closed #3, fix #4, fixes #5, fixed #6, resolve #7, resolves #8, resolved #9",
			[]IssueRef{bare(1), bare(2), bare(3), bare(4), bare(5), bare(6), bare(7), bare(8), bare(9)}},
		{"any case", "FIXES #1 and Closed #2", []IssueRef{bare(1), bare(2)}},
		{"keyword at end of subject", "Retry the link announcement. Fixes #4460", []IssueRef{bare(4460)}},
		{"colon after keyword", "Closes: #12", []IssueRef{bare(12)}},
		{"several spaces", "Resolves   #23.", []IssueRef{bare(23)}},
		{"newline before reference", "Fixes\n#24", []IssueRef{bare(24)}},
		{"inside punctuation", "- fixes #25)", []IssueRef{bare(25)}},
		{"qualified reference", "fixes openziti/ziti#2324", []IssueRef{in("openziti/ziti", 2324)}},
		{"issue url", "fixes https://github.com/openziti/ziti/issues/17", []IssueRef{in("openziti/ziti", 17)}},
		{"repeats are kept", "fixes #3, closes #3", []IssueRef{bare(3), bare(3)}},
		{"each reference needs a keyword", "Fixes #301 and #303", []IssueRef{bare(301)}},
		{"keyword inside a word", "prefixes #18, unfixed #19, suffixes #20", nil},
		{"no space before reference", "Fixed#21", nil},
		{"other keywords", "For #22, see #23, refs #24", nil},
		{"digits run into letters", "fixes #12abc", nil},
		{"pull request url", "fixes https://github.com/openziti/ziti/pull/17", nil},
		{"no references", "Update the changelog", nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, ClosingIssueRefs(test.text))
		})
	}
}

func TestIssueRefInRepo(t *testing.T) {
	req := require.New(t)
	req.True(IssueRef{Number: 1}.InRepo("openziti/ziti"))
	req.True(IssueRef{Repo: "openziti/ziti", Number: 1}.InRepo("openziti/ziti"))
	req.True(IssueRef{Repo: "OpenZiti/Ziti", Number: 1}.InRepo("openziti/ziti"))
	req.False(IssueRef{Repo: "openziti/sdk-golang", Number: 1}.InRepo("openziti/ziti"))
}

func TestIssueNumbersInRepo(t *testing.T) {
	refs := ClosingIssueRefs("fixes #12, fixes openziti/ziti#3, fixes openziti/sdk-golang#7, " +
		"closes https://github.com/openziti/ziti/issues/12, resolves #5")
	require.Equal(t, []int{3, 5, 12}, issueNumbersInRepo(refs, "openziti/ziti"))
	require.Nil(t, issueNumbersInRepo(nil, "openziti/ziti"))
}
