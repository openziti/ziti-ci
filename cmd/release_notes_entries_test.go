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
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/require"
)

const existingSection = `* github.com/openziti/channel/v5: [v4.3.11 -> v5.0.15](https://github.com/openziti/channel/compare/v4.3.11...v5.0.15)
    * [Issue #269](https://github.com/openziti/channel/issues/269) - Make channel logging pluggable

* github.com/openziti/identity: [v1.0.129 -> v1.0.137](https://github.com/openziti/identity/compare/v1.0.129...v1.0.137)
* github.com/openziti/ziti/v2: [v2.0.0 -> v2.1.0](https://github.com/openziti/ziti/compare/v2.0.0...v2.1.0)
    * [Issue #4137](https://github.com/openziti/ziti/issues/4137) - ziti tunnel ignores --dnsSvcIpRange <!-- keep -->
    * [Issue #4108](https://github.com/openziti/ziti/issues/4108) - Controller retains bbolt-managed memory past transaction
`

func TestParseIssueEntries(t *testing.T) {
	req := require.New(t)
	entries := parseIssueEntries(existingSection)
	req.Equal(3, len(entries))

	req.Equal("channel", entries[0].project)
	req.Equal("269", entries[0].issue)
	req.False(entries[0].pinned)

	req.Equal("ziti#4137", entries[1].key())
	req.True(entries[1].pinned)

	req.Equal("ziti#4108", entries[2].key())
	req.False(entries[2].pinned)
}

func TestMergePinnedEntries(t *testing.T) {
	req := require.New(t)

	// regenerated without either ziti entry, as happens when neither issue link is
	// discoverable from commits or pull requests
	generated := `* github.com/openziti/channel/v5: [v4.3.11 -> v5.0.15](https://github.com/openziti/channel/compare/v4.3.11...v5.0.15)
    * [Issue #269](https://github.com/openziti/channel/issues/269) - Make channel logging pluggable

* github.com/openziti/ziti/v2: [v2.0.0 -> v2.1.0](https://github.com/openziti/ziti/compare/v2.0.0...v2.1.0)
    * [Issue #4149](https://github.com/openziti/ziti/issues/4149) - Upgrading a running 1.x controller fails

`

	oldEntries := parseIssueEntries(existingSection)
	merged := mergePinnedEntries(generated, oldEntries)

	req.Contains(merged, "[Issue #4137]")
	req.NotContains(merged, "[Issue #4108]")

	// the pinned entry goes at the end of its own group, before the trailing blank line
	lines := strings.Split(merged, "\n")
	req.Equal("    * [Issue #4149](https://github.com/openziti/ziti/issues/4149) - Upgrading a running 1.x controller fails", lines[4])
	req.Equal("    * [Issue #4137](https://github.com/openziti/ziti/issues/4137) - ziti tunnel ignores --dnsSvcIpRange <!-- keep -->", lines[5])
	req.Equal("", lines[6])

	dropped := droppedEntries(oldEntries, merged)
	req.Equal(1, len(dropped))
	req.Equal("ziti#4108", dropped[0].key())
}

func TestMergePinnedEntryKeepsPosition(t *testing.T) {
	req := require.New(t)

	generated := `* github.com/openziti/ziti/v2: [v2.0.0 -> v2.1.0](https://github.com/openziti/ziti/compare/v2.0.0...v2.1.0)
    * [Issue #4149](https://github.com/openziti/ziti/issues/4149) - Upgrading a running 1.x controller fails
    * [Issue #4108](https://github.com/openziti/ziti/issues/4108) - Controller retains bbolt-managed memory past transaction

`

	merged := mergePinnedEntries(generated, parseIssueEntries(existingSection))
	lines := strings.Split(merged, "\n")

	// the pin preceded #4108 before regeneration, so it goes back ahead of it
	req.Contains(lines[1], "[Issue #4149]")
	req.Contains(lines[2], "[Issue #4137]")
	req.Contains(lines[3], "[Issue #4108]")
}

func TestMergePinnedEntryAlreadyFoundByScan(t *testing.T) {
	req := require.New(t)

	generated := `* github.com/openziti/ziti/v2: [v2.0.0 -> v2.1.0](https://github.com/openziti/ziti/compare/v2.0.0...v2.1.0)
    * [Issue #4137](https://github.com/openziti/ziti/issues/4137) - raw issue title from github

`
	merged := mergePinnedEntries(generated, parseIssueEntries(existingSection))

	// listed once, keeping the pinned wording rather than the generated one
	req.Equal(1, strings.Count(merged, "[Issue #4137]"))
	req.Contains(merged, "ziti tunnel ignores --dnsSvcIpRange "+keepMarker)
	req.NotContains(merged, "raw issue title from github")
}

func TestMergePinnedEntryWithNoGroup(t *testing.T) {
	req := require.New(t)

	pinned := parseIssueEntries(`    * [Issue #269](https://github.com/openziti/channel/issues/269) - Pluggable logging ` + keepMarker)
	generated := "* github.com/openziti/ziti/v2: [v2.0.0 -> v2.1.0](https://github.com/openziti/ziti/compare/v2.0.0...v2.1.0)\n\n"

	merged := mergePinnedEntries(generated, pinned)
	req.Contains(merged, "* github.com/openziti/channel\n")
	req.Contains(merged, "[Issue #269]")
}

func TestPreviousMinorRelease(t *testing.T) {
	req := require.New(t)

	versions := func(s ...string) []*version.Version {
		var result []*version.Version
		for _, each := range s {
			v, err := version.NewVersion(each)
			req.NoError(err)
			result = append(result, v)
		}
		return result
	}
	previous := func(next string, tags ...string) string {
		v, err := version.NewVersion(next)
		req.NoError(err)
		result := previousMinorRelease(versions(tags...), v)
		if result == nil {
			return ""
		}
		return result.String()
	}

	// patch tags on release branches are never the basis for a new minor's notes
	req.Equal("2.0.0", previous("2.1.0", "1.6.6", "2.0.0", "2.0.1", "2.0.2"))
	req.Equal("2.7.0", previous("3.0.0", "2.6.0", "2.7.0", "2.7.4"))
	req.Equal("", previous("1.0.0", "1.0.0", "1.0.1"))
}

func TestSameMinor(t *testing.T) {
	req := require.New(t)

	v := func(s string) *version.Version {
		result, err := version.NewVersion(s)
		req.NoError(err)
		return result
	}

	req.True(sameMinor(v("2.1.3"), v("2.1.4")))
	req.False(sameMinor(v("2.0.2"), v("2.1.0")))
	req.False(sameMinor(v("1.1.0"), v("2.1.0")))
}

func TestPullRequestFromCommit(t *testing.T) {
	req := require.New(t)
	pr := func(msg string) string {
		return pullRequestFromCommit(&object.Commit{Message: msg})
	}

	req.Equal("4053", pr("Merge pull request #4053 from openziti/issue-4052-use-refresh-token-to-reauth\n\nuse refresh token to reauth"))
	req.Equal("4131", pr("set dns ip range before creating interceptor (#4131)"))
	req.Equal("", pr("Copy terminator peer data out of bolt memory. Fixes #4108"))
	req.Equal("", pr("mentions (#4131) but not at the end of the subject"))
}

func TestIssueFromMergeBranch(t *testing.T) {
	req := require.New(t)
	issue := func(msg string) string {
		return issueFromMergeBranch(&object.Commit{Message: msg})
	}

	req.Equal("4052", issue("Merge pull request #4053 from openziti/issue-4052-use-refresh-token-to-reauth"))
	req.Equal("4039", issue("Merge pull request #4040 from openziti/issue-4039-fix-cached-login-error"))
	req.Equal("", issue("Merge pull request #4109 from openziti/fix-terminator-peerdata-mmap"))
	req.Equal("", issue("regular commit on issue-4052-branch"))
}
