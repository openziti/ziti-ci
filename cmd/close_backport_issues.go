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
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var releaseBranchRegex = regexp.MustCompile(`^release-v(\d+(?:\.\d+)?)\.x$`)
var changelogReleaseRegex = regexp.MustCompile(`(?m)^# Release (\d+(?:\.\d+)*)\s*$`)

// backportPrefix returns the title prefix of the backport tracking issues for a release branch:
// [Backport-2.0] for release-v2.0.x and [Backport-4.x] for release-v4.x. It returns false for a
// branch that isn't named like a release branch.
func backportPrefix(branch string) (string, bool) {
	match := releaseBranchRegex.FindStringSubmatch(branch)
	if match == nil {
		return "", false
	}
	line := match[1]
	if !strings.Contains(line, ".") {
		line += ".x"
	}
	return "[Backport-" + line + "]", true
}

type closeBackportIssuesCmd struct {
	BaseCommand
	repo             string
	token            string
	commentOnSkipped bool
	api              githubApi
}

// Init records the arguments. Unlike BaseCommand.Init it needs no checkout or version file.
func (cmd *closeBackportIssuesCmd) Init(args []string) {
	cmd.Args = args
}

func (cmd *closeBackportIssuesCmd) Execute() error {
	if cmd.api == nil {
		if cmd.repo == "" {
			cmd.repo = os.Getenv("GITHUB_REPOSITORY")
		}
		if cmd.repo == "" {
			return errors.New("no repository given, and GITHUB_REPOSITORY is not set")
		}
		if cmd.token == "" {
			cmd.token = os.Getenv("GITHUB_TOKEN")
		}
		if cmd.token == "" {
			cmd.token = os.Getenv("GH_TOKEN")
		}
		if cmd.token == "" {
			return errors.New("no token given, and neither GITHUB_TOKEN nor GH_TOKEN is set")
		}
		cmd.api = newGithubRestApi(cmd.repo, cmd.token)
	}

	var errs []error
	for _, arg := range cmd.Args {
		number, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
		if err != nil {
			return fmt.Errorf("invalid pull request number %q", arg)
		}
		if err = cmd.processPullRequest(number); err != nil {
			cmd.annotate("error", "#%d: %v", number, err)
			errs = append(errs, fmt.Errorf("#%d: %w", number, err))
		}
	}
	return errors.Join(errs...)
}

// processPullRequest closes the open issues that a pull request merged to a release branch
// references with a closing keyword, provided their titles carry that branch's backport prefix.
// References to other issues are reported and, with commentOnSkipped, listed in a comment on the
// pull request.
func (cmd *closeBackportIssuesCmd) processPullRequest(number int) error {
	pr, err := cmd.api.GetPullRequest(number)
	if err != nil {
		return err
	}
	if !pr.Merged {
		cmd.annotate("notice", "#%d is not merged, skipping", number)
		return nil
	}
	prefix, ok := backportPrefix(pr.Base.Ref)
	if !ok {
		cmd.annotate("notice", "#%d targets %s, which is not a release branch, skipping", number, pr.Base.Ref)
		return nil
	}

	messages, err := cmd.api.GetPullRequestCommitMessages(number)
	if err != nil {
		return err
	}
	refs := ClosingIssueRefs(pr.Body + "\n" + strings.Join(messages, "\n"))
	for _, ref := range refs {
		if !ref.InRepo(cmd.repo) {
			cmd.annotate("notice", "#%d references %s#%d in another repository, skipping", number, ref.Repo, ref.Number)
		}
	}
	issueNumbers := issueNumbersInRepo(refs, cmd.repo)
	if len(issueNumbers) == 0 {
		cmd.annotate("notice", "#%d has no closing references to issues in %s", number, cmd.repo)
		return nil
	}

	closeComment := fmt.Sprintf("Fixed on `%s` by #%d (commit %s)%s.",
		pr.Base.Ref, number, shortSha(pr.MergeCommitSha), cmd.pendingReleaseNote(pr.MergeCommitSha))

	var skipped []string
	var errs []error
	for _, issueNumber := range issueNumbers {
		issue, err := cmd.api.GetIssue(issueNumber)
		if err != nil {
			cmd.annotate("warning", "#%d references #%d, which could not be read: %v", number, issueNumber, err)
			errs = append(errs, err)
			continue
		}

		switch {
		case issue.IsPullRequest():
			cmd.annotate("notice", "#%d references #%d, which is a pull request, skipping", number, issueNumber)
		case !strings.HasPrefix(issue.Title, prefix):
			cmd.annotate("warning", "#%d references #%d (%s), which is not a %s issue, not closing it",
				number, issueNumber, issue.Title, prefix)
			skipped = append(skipped, fmt.Sprintf("- #%d %s", issueNumber, issue.Title))
		case issue.State == "closed":
			cmd.annotate("notice", "#%d is already closed", issueNumber)
		default:
			cmd.Infof("closing #%d (%s): %s\n", issueNumber, issue.Title, closeComment)
			if !cmd.dryRun {
				if err = cmd.api.CommentAndClose(issueNumber, closeComment); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	if len(skipped) > 0 && cmd.commentOnSkipped {
		body := fmt.Sprintf("These closing references are not `%s` issues, so they were not closed:\n\n%s\n\n"+
			"A pull request to `%s` should reference that branch's `%s` tracking issue.",
			prefix, strings.Join(skipped, "\n"), pr.Base.Ref, prefix)
		cmd.Infof("commenting on #%d:\n%s\n", number, body)
		if !cmd.dryRun {
			if err = cmd.api.Comment(number, body); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

// pendingReleaseNote returns ", to be released in x.y.z" when the changelog at ref opens with a
// release that has no v-prefixed tag yet, or an empty string otherwise.
func (cmd *closeBackportIssuesCmd) pendingReleaseNote(ref string) string {
	changelog, found, err := cmd.api.GetFile("CHANGELOG.md", ref)
	if err != nil {
		cmd.annotate("warning", "unable to read CHANGELOG.md at %s: %v", ref, err)
		return ""
	}
	if !found {
		return ""
	}
	match := changelogReleaseRegex.FindStringSubmatch(changelog)
	if match == nil {
		return ""
	}
	released, err := cmd.api.TagExists("v" + match[1])
	if err != nil {
		cmd.annotate("warning", "unable to check for tag v%s: %v", match[1], err)
		return ""
	}
	if released {
		return ""
	}
	return ", to be released in " + match[1]
}

// annotate prints a message as a GitHub Actions annotation of the given level (notice, warning or
// error) when running in Actions, and as plain output otherwise.
func (cmd *closeBackportIssuesCmd) annotate(level string, format string, params ...interface{}) {
	msg := fmt.Sprintf(format, params...)
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		_, _ = fmt.Fprintf(cmd.Cmd.OutOrStdout(), "::%s::%s\n", level, msg)
	} else {
		_, _ = fmt.Fprintf(cmd.Cmd.OutOrStdout(), "%s: %s\n", strings.ToUpper(level), msg)
	}
}

func shortSha(sha string) string {
	if len(sha) > 9 {
		return sha[:9]
	}
	return sha
}

func newCloseBackportIssuesCmd(root *RootCommand) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "close-backport-issues <pull request number>...",
		Short: "Close the [Backport-x.y] issues referenced by pull requests merged to release branches",
		Long: "GitHub only acts on closing keywords in pull requests to the default branch. This closes the " +
			"open issues that each given pull request, once merged to a release-vX.Y.x or release-vX.x branch, " +
			"references with a closing keyword in its description or commit messages, provided the issue title " +
			"starts with that branch's [Backport-X.Y] or [Backport-X.x] prefix. Use --dry-run to report without " +
			"changing anything.",
		Args: cobra.MinimumNArgs(1),
	}

	result := &closeBackportIssuesCmd{
		BaseCommand: BaseCommand{
			RootCommand: root,
			Cmd:         cobraCmd,
		},
	}

	cobraCmd.Flags().StringVar(&result.repo, "repo", "", "repository as owner/name (default $GITHUB_REPOSITORY)")
	cobraCmd.Flags().StringVar(&result.token, "token", "", "GitHub token (default $GITHUB_TOKEN, then $GH_TOKEN)")
	cobraCmd.Flags().BoolVar(&result.commentOnSkipped, "comment-on-skipped", false,
		"comment on the pull request when it references issues that are not its branch's backport issues")

	return FinalizeErroringCmd(result)
}
