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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// The current checkout sits beside one named after the project, as a worktree sits beside its main
// checkout. Each has one commit after the v1.0.0 tag, closing a different issue.
func setUpCheckouts(t *testing.T) (current string) {
	t.Helper()
	parent := t.TempDir()
	initTestRepo(t, filepath.Join(parent, "sdk-golang"), "Change in the sibling checkout. Fixes #2")
	current = filepath.Join(parent, "sdk-golang-1.8.x-worktree")
	initTestRepo(t, current, "Change in the current checkout. Fixes #1")
	installFakeGh(t)
	return current
}

// initTestRepo creates a repository at dir with a v1.0.0 tag and a commit after it carrying message.
func initTestRepo(t *testing.T, dir string, message string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0755))
	git := func(args ...string) {
		t.Helper()
		base := []string{"-C", dir, "-c", "user.name=Test Author", "-c", "user.email=test@example.com",
			"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}
		out, err := exec.Command("git", append(base, args...)...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
	git("init", "-q")
	git("commit", "-q", "--allow-empty", "-m", "Release")
	git("tag", "v1.0.0")
	git("commit", "-q", "--allow-empty", "-m", message)
}

// installFakeGh puts a gh first on the PATH that answers `gh issue view <n>` with a link to issue n.
func installFakeGh(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"[Issue #$3](https://github.com/openziti/sdk-golang/issues/$3) - Test issue $3\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "gh"), []byte(script), 0755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newTestReleaseNotesCmd(out *bytes.Buffer) *baseBuildReleaseNotesCmd {
	return &baseBuildReleaseNotesCmd{
		BaseCommand: BaseCommand{
			RootCommand: &RootCommand{quiet: true, noFetch: true},
			Cmd:         &cobra.Command{},
		},
		NoPrScan: true,
		Writer:   out,
	}
}

func TestGetOwnChangesReadsTheCurrentCheckout(t *testing.T) {
	req := require.New(t)
	t.Chdir(setUpCheckouts(t))

	out := &bytes.Buffer{}
	req.NoError(newTestReleaseNotesCmd(out).GetOwnChanges("sdk-golang", "v1.0.0", "HEAD"))
	req.Contains(out.String(), "Issue #1")
	req.NotContains(out.String(), "Issue #2")
}

func TestGetChangesReadsTheSiblingCheckout(t *testing.T) {
	req := require.New(t)
	t.Chdir(setUpCheckouts(t))

	out := &bytes.Buffer{}
	req.NoError(newTestReleaseNotesCmd(out).GetChanges("sdk-golang", "v1.0.0", "HEAD"))
	req.Contains(out.String(), "Issue #2")
	req.NotContains(out.String(), "Issue #1")
}
