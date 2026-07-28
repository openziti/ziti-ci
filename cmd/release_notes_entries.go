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
	"regexp"
	"strings"
)

// keepMarker marks an issue entry that should survive regeneration of a release notes
// section. It is an HTML comment so that it stays invisible in rendered markdown.
const keepMarker = "<!-- keep -->"

var issueEntryRegex = regexp.MustCompile(`^\s*\*\s+\[Issue #(\d+)]\(https://github\.com/openziti/([^/]+)/issues/\d+\)`)
var moduleGroupRegex = regexp.MustCompile(`^\* +github\.com/openziti/([^/:]+)`)

// issueEntry is an issue bullet parsed out of a release notes section.
type issueEntry struct {
	project string
	issue   string
	line    string
	pinned  bool
}

// key identifies the issue an entry refers to, so entries can be compared across
// regenerations regardless of how the title is worded.
func (e issueEntry) key() string {
	return e.project + "#" + e.issue
}

// parseIssueEntries returns the issue bullets in a release notes section, noting which
// ones are pinned with the keep marker.
func parseIssueEntries(section string) []issueEntry {
	var result []issueEntry
	for _, line := range strings.Split(section, "\n") {
		match := issueEntryRegex.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		result = append(result, issueEntry{
			project: match[2],
			issue:   match[1],
			line:    strings.TrimRight(line, " \t"),
			pinned:  strings.Contains(line, keepMarker),
		})
	}
	return result
}

// mergePinnedEntries restores pinned entries in a regenerated section. A pin means the line
// is hand-maintained: it is put back if the scan didn't find the issue, and it keeps its own
// wording if the scan did, since a pinned entry is often worded better than the issue title
// it was derived from.
func mergePinnedEntries(section string, entries []issueEntry) string {
	present := map[string]struct{}{}
	for _, e := range parseIssueEntries(section) {
		present[e.key()] = struct{}{}
	}

	// walking backwards lets a run of adjacent pins anchor to each other, since the entry a
	// pin anchors to is always inserted before the pin itself
	lines := strings.Split(section, "\n")
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !e.pinned {
			continue
		}
		if _, found := present[e.key()]; found {
			replaceIssueEntry(lines, e)
			continue
		}
		lines = insertIssueEntry(lines, e, nextEntryForProject(entries, i))
		present[e.key()] = struct{}{}
	}
	return strings.Join(lines, "\n")
}

// replaceIssueEntry swaps the generated line for an issue with the pinned one, leaving it
// where the scan put it.
func replaceIssueEntry(lines []string, e issueEntry) {
	for i, line := range lines {
		if entryKey(line) == e.key() {
			lines[i] = e.line
			return
		}
	}
}

// nextEntryForProject returns the key of the entry that followed the given one in the same
// project, or an empty string if it was last.
func nextEntryForProject(entries []issueEntry, idx int) string {
	for i := idx + 1; i < len(entries); i++ {
		if entries[i].project == entries[idx].project {
			return entries[i].key()
		}
	}
	return ""
}

// insertIssueEntry puts an issue bullet back into its project's group, ahead of the entry
// it preceded before regeneration so that a curated order survives, or at the end of the
// group if that entry is gone. If the project has no group, which happens when its version
// didn't change this release, a bare group is added so that a pinned entry is never dropped
// for lack of a home.
func insertIssueEntry(lines []string, e issueEntry, anchor string) []string {
	start := -1
	for i, line := range lines {
		if match := moduleGroupRegex.FindStringSubmatch(line); match != nil && match[1] == e.project {
			start = i
			break
		}
	}

	if start == -1 {
		return append(lines, fmt.Sprintf("* github.com/openziti/%v", e.project), e.line, "")
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if moduleGroupRegex.MatchString(lines[i]) {
			end = i
			break
		}
	}

	insertAt := -1
	if anchor != "" {
		for i := start + 1; i < end; i++ {
			if entryKey(lines[i]) == anchor {
				insertAt = i
				break
			}
		}
	}

	if insertAt == -1 {
		insertAt = end
		for insertAt > start+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
			insertAt--
		}
	}

	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertAt]...)
	result = append(result, e.line)
	return append(result, lines[insertAt:]...)
}

// entryKey returns the issue key for a changelog line, or an empty string if the line isn't
// an issue bullet.
func entryKey(line string) string {
	match := issueEntryRegex.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	return match[2] + "#" + match[1]
}

// droppedEntries returns the entries that were in the old section but aren't in the newly
// generated one. Not every issue link is discoverable from commits and pull requests, so
// regeneration can otherwise lose hand-added entries without saying so.
func droppedEntries(oldEntries []issueEntry, newSection string) []issueEntry {
	present := map[string]struct{}{}
	for _, e := range parseIssueEntries(newSection) {
		present[e.key()] = struct{}{}
	}

	var result []issueEntry
	for _, e := range oldEntries {
		if _, found := present[e.key()]; !found {
			result = append(result, e)
		}
	}
	return result
}

// reportDropped warns about entries removed by regeneration and how to keep them.
func (cmd *baseBuildReleaseNotesCmd) reportDropped(dropped []issueEntry) {
	if len(dropped) == 0 {
		return
	}

	var keys []string
	for _, e := range dropped {
		keys = append(keys, e.key())
	}

	cmd.Warnf("%v issue entries were not found by the commit and pull request scan and have been removed: %v\n",
		len(dropped), strings.Join(keys, ", "))
	cmd.Warnf("to keep an entry across regeneration, add %v to the end of its line\n", keepMarker)
}
