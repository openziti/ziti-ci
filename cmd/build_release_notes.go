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
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

type baseBuildReleaseNotesCmd struct {
	BaseCommand
	AllCommits    bool
	ShowUnchanged bool
	NoPrScan      bool
	StartVersion  string
	Writer        io.Writer

	// merged pull requests by project and number, listed on demand. GetChanges scans one
	// project at a time and sets the cutoff for the project it's working on.
	pullRequests map[string]map[string]string
	prCutoff     string
}

func (cmd *baseBuildReleaseNotesCmd) getWriter() io.Writer {
	if cmd.Writer != nil {
		return cmd.Writer
	}
	return os.Stdout
}

func (cmd *baseBuildReleaseNotesCmd) printf(format string, args ...interface{}) {
	fmt.Fprintf(cmd.getWriter(), format, args...)
}

type buildReleaseNotesCmd struct {
	baseBuildReleaseNotesCmd
}

func (cmd *baseBuildReleaseNotesCmd) getUnversionedPath(m module.Version) string {
	parts := strings.Split(m.Path, "/")
	lastElement := parts[len(parts)-1]
	match, err := regexp.Match(`v(\d+)`, []byte(lastElement))
	if err != nil {
		panic(err)
	}
	if match {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return m.Path
}

func (cmd *baseBuildReleaseNotesCmd) getPreviousVersion(path string) *string {
	parts := strings.Split(path, "/")
	lastElement := parts[len(parts)-1]
	match, err := regexp.Match(`^v(\d+)`, []byte(lastElement))
	if err != nil {
		panic(err)
	}
	if match {
		base := strings.Join(parts[:len(parts)-1], "/")
		versionStr := lastElement[1:]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			panic(err)
		}
		if version == 2 {
			return &base
		}
		base = fmt.Sprintf("%v/v%v", base, version-1)
		return &base
	}
	return nil
}

func (cmd *buildReleaseNotesCmd) Execute() {
	if !cmd.RootCobraCmd.Flags().Changed("quiet") {
		cmd.quiet = true
	}
	cmd.initVersions()
	cmd.generateReleaseNotes()
}

func (cmd *buildReleaseNotesCmd) initVersions() {
	if cmd.StartVersion != "" {
		v, err := version.NewVersion(cmd.StartVersion)
		if err != nil {
			panic(err)
		}
		cmd.CurrentVersion = v
		cmd.EvalCurrentAndNextVersion()
		return
	}

	cmd.EvalCurrentAndNextVersion()
	cmd.evalStartVersion()
}

func (cmd *buildReleaseNotesCmd) generateReleaseNotes() {
	cmd.Infof("generating release notes %v -> %v\n", cmd.CurrentVersion, cmd.NextVersion)

	data, err := os.ReadFile("go.mod")
	if err != nil {
		panic(err)
	}

	newGoMod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		panic(err)
	}

	output := cmd.runCommandWithOutput("get go.mod contents", "git", "show", fmt.Sprintf("v%v:go.mod", cmd.CurrentVersion))
	data = []byte(strings.Join(output, "\n"))
	oldGoMod, err := modfile.Parse("go.mod", data, nil)
	if err != nil {
		panic(err)
	}

	oldVersions := map[string]*modfile.Require{}

	for _, m := range oldGoMod.Require {
		if strings.Contains(m.Mod.Path, "openziti") {
			oldVersions[m.Mod.Path] = m
		}
	}

	for _, m := range newGoMod.Require {
		if strings.Contains(m.Mod.Path, "openziti") {
			cmd.Infof("checking dependency %v\n", m.Mod.Path)
			project := strings.Split(m.Mod.Path, "/")[2]
			prev, found := oldVersions[m.Mod.Path]
			if !found {
				path := m.Mod.Path
				prevVersion := cmd.getPreviousVersion(path)
				for prevVersion != nil {
					prev, found = oldVersions[*prevVersion]
					if found {
						break
					}
					prevVersion = cmd.getPreviousVersion(*prevVersion)
				}
			}
			if !found {
				cmd.printf("* %v: %v (new)\n", m.Mod.Path, m.Mod.Version)
			} else if m.Mod.Version != prev.Mod.Version {
				cmd.printf("* %v: [%v -> %v](https://github.com/openziti/%v/compare/%v...%v)\n", m.Mod.Path, prev.Mod.Version, m.Mod.Version, project, prev.Mod.Version, m.Mod.Version)
				if err = cmd.GetChanges(project, prev.Mod.Version, m.Mod.Version); err != nil {
					panic(err)
				}
			} else if cmd.ShowUnchanged {
				cmd.printf("* %v: %v (unchanged)\n", m.Mod.Path, m.Mod.Version)
			}
		}
	}

	cmd.Infof("checking dependency %v\n", newGoMod.Module.Mod.Path)
	cmd.printf("* %v: [v%v -> v%v](https://github.com/openziti/ziti/compare/v%v...v%v)\n",
		newGoMod.Module.Mod.Path, cmd.CurrentVersion, cmd.NextVersion, cmd.CurrentVersion, cmd.NextVersion)
	if err = cmd.GetChanges("ziti", "v"+cmd.CurrentVersion.String(), "HEAD"); err != nil {
		panic(err)
	}
}

// evalStartVersion picks the release that notes should be diffed against. Notes for a patch
// release cover the changes since the previous patch, and notes for the first release of a
// minor cover everything since the previous minor release. Version evaluation instead falls
// back to the newest tag of any kind, which on a minor release picks up a patch tag from a
// release branch whose commits never reached this branch, leaving nothing for the commit walk
// to stop at.
func (cmd *baseBuildReleaseNotesCmd) evalStartVersion() {
	if cmd.CurrentVersion != nil && sameMinor(cmd.CurrentVersion, cmd.NextVersion) {
		return
	}

	if previous := previousMinorRelease(cmd.getVersionList("tag", "--list"), cmd.NextVersion); previous != nil {
		cmd.CurrentVersion = previous
	}
}

// previousMinorRelease returns the newest x.y.0 release older than the given version.
func previousMinorRelease(versions []*version.Version, next *version.Version) *version.Version {
	var result *version.Version
	for _, v := range versions {
		if v.Segments()[2] != 0 || !v.LessThan(next) {
			continue
		}
		if result == nil || result.LessThan(v) {
			result = v
		}
	}
	return result
}

// sameMinor reports whether two versions are in the same major.minor release line.
func sameMinor(v1 *version.Version, v2 *version.Version) bool {
	return v1.Segments()[0] == v2.Segments()[0] && v1.Segments()[1] == v2.Segments()[1]
}

func (cmd *baseBuildReleaseNotesCmd) GetChanges(project string, oldVersion string, newVersion string) error {
	cmd.Infof("  scanning changes for %v (%v -> %v)\n", project, oldVersion, newVersion)
	dir, err := os.Getwd()
	if err != nil {
		return errors.Wrapf(err, "unable to get working directory")
	}

	defer func() {
		if err := os.Chdir(dir); err != nil {
			panic(errors.Wrapf(err, "unable to restore working directory to %v", dir))
		}
	}()

	if err := os.Chdir("../" + project); err != nil {
		return errors.Wrapf(err, "")
	}

	if !cmd.noFetch {
		cmd.runGitCommandAlways("fetch latest tags", "fetch", "--tags")
	}

	r, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return err
	}

	newTagHash, err := r.ResolveRevision(plumbing.Revision(newVersion))
	if err != nil {
		// check if we're pointing to git hash
		parts := strings.Split(newVersion, "-")
		if len(parts) == 3 {
			gitHash := parts[2]
			newTagHash, err = r.ResolveRevision(plumbing.Revision(gitHash))
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	oldTagHash, err := r.ResolveRevision(plumbing.Revision(oldVersion))
	if err != nil {
		return err
	}

	oldTagIter, err := r.Log(&git.LogOptions{Order: git.LogOrderCommitterTime, From: *oldTagHash})
	if err != nil {
		return err
	}
	defer oldTagIter.Close()

	tagCommit, err := oldTagIter.Next()
	if err != nil {
		return err
	}

	// The old tag may be a ziti-ci version-bump commit not in the main-line, so find its parent
	if tagCommit.NumParents() == 1 && tagCommit.Author.Name == "ziti-ci" {
		tagCommit, err = tagCommit.Parent(0)
		if err != nil {
			return err
		}
		oldTagHash = &tagCommit.Hash
	}

	// back-date the pull request search, since a cherry-picked commit can land after the tag
	// while the pull request it names was merged well before it
	cmd.prCutoff = tagCommit.Committer.When.AddDate(0, 0, -60).Format("2006-01-02")

	iter, err := r.Log(&git.LogOptions{Order: git.LogOrderCommitterTime, From: *newTagHash})
	if err != nil {
		return err
	}

	showedChange := false

	defer iter.Close()
	defer func() {
		if showedChange {
			cmd.printf("\n")
		}
	}()

	issues := map[string]struct{}{}

	for {
		c, err := iter.Next()
		if err == io.EOF {
			return nil
		}
		if c == nil {
			return err
		}
		if c.Hash == *oldTagHash {
			return nil
		}

		if c.Author.Name == "ziti-ci" || c.Author.Name == "dependabot[bot]" {
			continue
		}

		commitIssues := cmd.extractIssues(project, c)
		if len(commitIssues) == 0 {
			commitIssues = cmd.extractPullRequestIssues(project, c)
		}

		issueFound := false
		for _, issue := range commitIssues {
			if _, ok := issues[issue]; !ok {
				cmd.outputIssue(issue)
				showedChange = true
				issueFound = true
				issues[issue] = struct{}{}
			}
		}

		// merge commits duplicate work already listed by the commits they bring in, so
		// they're only useful for the issue links they carry
		if !issueFound && cmd.AllCommits && c.NumParents() < 2 {
			lines := strings.Split(c.Message, "\n")
			cmd.printf("    * %v: %v (%v)\n", c.Hash.String()[:7], lines[0], c.Author.Email)
			showedChange = true
		}

	}
}

func (cmd *baseBuildReleaseNotesCmd) extractIssues(project string, c *object.Commit) []string {
	return cmd.extractIssuesFromText(project, c.Message)
}

// extractIssuesFromText returns the issues that the given text claims to close, either as
// a bare issue number or one qualified with the project's repository.
func (cmd *baseBuildReleaseNotesCmd) extractIssuesFromText(project string, text string) []string {
	r, err := regexp.Compile(`(fix(e[sd])?|close[sd]?|resolve[sd]?)\s*(openziti/` + regexp.QuoteMeta(project) + `)?#(\d+)`)
	if err != nil {
		panic(err)
	}

	matches := r.FindAllStringSubmatch(strings.ToLower(text), -1)
	var result []string
	for _, match := range matches {
		result = append(result, match[4])
	}
	return result
}

// extractPullRequestIssues looks for issue links that a commit message doesn't carry
// itself, escalating from the pull request title and body to the source branch name. Merge
// and squash-merge commits often name only the pull request, leaving the issue link in the
// pull request description where a commit-only scan can't see it.
func (cmd *baseBuildReleaseNotesCmd) extractPullRequestIssues(project string, c *object.Commit) []string {
	if cmd.NoPrScan {
		return nil
	}

	if pr := pullRequestFromCommit(c); pr != "" {
		if text, found := cmd.getPullRequests(project)[pr]; found {
			if result := cmd.extractIssuesFromText(project, text); len(result) > 0 {
				return result
			}
		}
	}

	if issue := issueFromMergeBranch(c); issue != "" {
		return []string{issue}
	}

	return nil
}

var mergeCommitPrRegex = regexp.MustCompile(`^Merge pull request #(\d+)`)
var squashCommitPrRegex = regexp.MustCompile(`\(#(\d+)\)\s*$`)
var mergeBranchIssueRegex = regexp.MustCompile(`^Merge pull request #\d+ from [^/\s]+/issue[-_]?(\d+)`)

// pullRequestFromCommit returns the pull request a commit came from, taken from the
// standard GitHub merge message or from the trailing (#nnn) that squash merges leave in the
// subject.
func pullRequestFromCommit(c *object.Commit) string {
	subject := strings.SplitN(c.Message, "\n", 2)[0]
	if match := mergeCommitPrRegex.FindStringSubmatch(subject); match != nil {
		return match[1]
	}
	if match := squashCommitPrRegex.FindStringSubmatch(subject); match != nil {
		return match[1]
	}
	return ""
}

// issueFromMergeBranch returns the issue number embedded in a merge commit's source branch
// name, the last resort for pull requests that never spelled the link out.
func issueFromMergeBranch(c *object.Commit) string {
	subject := strings.SplitN(c.Message, "\n", 2)[0]
	if match := mergeBranchIssueRegex.FindStringSubmatch(subject); match != nil {
		return match[1]
	}
	return ""
}

// getPullRequests returns the merged pull requests for a project, keyed by number, with the
// title and body flattened into one searchable string. The whole set is listed in a single
// call the first time it's needed, since looking up pull requests one commit at a time makes
// generating release notes take minutes.
func (cmd *baseBuildReleaseNotesCmd) getPullRequests(project string) map[string]string {
	if result, found := cmd.pullRequests[project]; found {
		return result
	}

	cmd.Infof("  listing merged pull requests for %v since %v\n", project, cmd.prCutoff)
	bin, err := exec.LookPath("gh")
	if err != nil {
		panic(errors.Wrap(err, "gh (github CLI) not found. Please make sure it's installed an you are authenticated"))
	}

	result := map[string]string{}
	out, err := cmd.runCommandWithOutputFailOptional(false, "List PRs", bin,
		"pr", "list", "--state", "merged", "--limit", "1000",
		"--search", "merged:>="+cmd.prCutoff,
		"--json", "number,title,body",
		"--jq", `.[] | "\(.number)\t" + ((.title + " " + (.body // "")) | gsub("[\\r\\n]+"; " "))`)
	if err == nil {
		for _, line := range out {
			if number, text, found := strings.Cut(line, "\t"); found {
				result[number] = text
			}
		}
	}

	if cmd.pullRequests == nil {
		cmd.pullRequests = map[string]map[string]string{}
	}
	cmd.pullRequests[project] = result

	return result
}

func (cmd *baseBuildReleaseNotesCmd) outputIssue(issue string) {
	cmd.Infof("  looking up issue #%v\n", issue)
	bin, err := exec.LookPath("gh")
	if err != nil {
		panic(errors.Wrap(err, "gh (github CLI) not found. Please make sure it's installed an you are authenticated"))
	}
	out, err := cmd.runCommandWithOutputFailOptional(false, "Get Issue", bin,
		"issue", "view", issue, "--json", "number,title,url", "--jq", `"[Issue #" + (.number|tostring) + "](" + .url + ") - " + .title`)
	if err != nil || len(out) == 0 {
		return
	}

	// gh resolves pull request numbers as well as issue numbers, and references picked up
	// from pull request descriptions are often to other pull requests
	if strings.Contains(out[0], "/pull/") {
		cmd.Infof("  #%v is a pull request, not an issue, skipping\n", issue)
		return
	}

	cmd.printf("    * %v\n", out[0])
}

func newBuildReleaseNotesCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "build-release-notes",
		Short: "Prints out the release notes for the latest or a given version",
		Args:  cobra.MaximumNArgs(1),
	}

	result := &buildReleaseNotesCmd{
		baseBuildReleaseNotesCmd: baseBuildReleaseNotesCmd{
			BaseCommand: BaseCommand{
				RootCommand: root,
				Cmd:         cobraCmd,
			},
		},
	}

	cobraCmd.Flags().BoolVarP(&result.AllCommits, "all-commits", "a", false, "Show all commits, not just closed issues")
	cobraCmd.Flags().BoolVarP(&result.ShowUnchanged, "show-unchanged", "u", false, "Show OpenZiti upstream libraries, even if unchanged")
	cobraCmd.Flags().BoolVar(&result.NoPrScan, "no-pr-scan", false, "Don't inspect pull requests for issue links missing from commit messages")
	cobraCmd.Flags().StringVarP(&result.StartVersion, "start-version", "s", "", "Version to diff against, instead of the previous release in the same minor, or the previous minor release")

	return Finalize(result)
}
