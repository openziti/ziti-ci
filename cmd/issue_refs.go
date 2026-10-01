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
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// IssueRef is a reference to a GitHub issue. Repo is the owner/name the reference was qualified
// with, or empty for a bare #N.
type IssueRef struct {
	Repo   string
	Number int
}

var closingRefRegex = regexp.MustCompile(`(?i)\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?):?\s+` +
	`(?:https://github\.com/([\w.-]+/[\w.-]+)/issues/(\d+)|([\w.-]+/[\w.-]+)?#(\d+))\b`)

// ClosingIssueRefs returns the issues text references with a GitHub closing keyword (close, fix or
// resolve, in any tense), in order of appearance and including repeats. Each reference needs its own
// keyword, so "fixes #1 and #2" references only #1. Qualified references (owner/name#N) and issue
// URLs carry their repository; bare #N references do not.
func ClosingIssueRefs(text string) []IssueRef {
	var result []IssueRef
	for _, match := range closingRefRegex.FindAllStringSubmatch(text, -1) {
		repo, number := match[3], match[4]
		if match[2] != "" {
			repo, number = match[1], match[2]
		}
		n, err := strconv.Atoi(number)
		if err != nil {
			continue
		}
		result = append(result, IssueRef{Repo: repo, Number: n})
	}
	return result
}

// InRepo reports whether the reference belongs to repo (owner/name, compared case-insensitively).
// A bare reference belongs to whichever repository it appears in, so it is in every repo.
func (self IssueRef) InRepo(repo string) bool {
	return self.Repo == "" || strings.EqualFold(self.Repo, repo)
}

// issueNumbersInRepo returns the distinct numbers of the refs in repo, in ascending order.
func issueNumbersInRepo(refs []IssueRef, repo string) []int {
	var result []int
	for _, ref := range refs {
		if ref.InRepo(repo) {
			result = append(result, ref.Number)
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}
