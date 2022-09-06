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
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExtractIssues(t *testing.T) {
	req := require.New(t)
	a := func(s ...string) []string {
		return s
	}
	req.Equal(a("10"), getIssues("Fixes #10"))
	req.Equal(a("12"), getIssues("This commit fixed #12"))
	req.Equal(a("13", "521"), getIssues("This commit fix #13 and FiXed #521"))
	req.Equal(a("20", "10", "5"), getIssues("This commit fixes #20, closes #10 and resolves #5"))
	req.Equal(a("20", "10", "5"), getIssues("This commit fix #20, close #10 and resolve #5"))
	req.Equal(a("20", "10", "5"), getIssues("This commit fixed #20, closed #10 and resolved #5"))
}

func getIssues(s string) []string {
	return (&buildReleaseNotesCmd{}).extractIssues(&object.Commit{
		Message: s,
	})
}
