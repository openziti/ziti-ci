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
	"github.com/hashicorp/go-version"
	"strconv"
	"strings"
)

const (
	Minor = 1
	Patch = 2
)

func setPatch(v *version.Version, patch int) *version.Version {
	parts := v.Segments()
	for len(parts) < 3 {
		parts = append(parts, 0)
	}
	parts[2] = patch
	return newVersion(parts)
}

func getNext(index int, v *version.Version) *version.Version {
	parts := v.Segments()
	for len(parts) < 3 && len(parts) < (index+1) {
		parts = append(parts, 0)
	}
	parts[index] = parts[index] + 1
	return newVersion(parts)
}

func newVersion(parts []int) *version.Version {
	var stringParts []string
	for _, part := range parts {
		stringParts = append(stringParts, strconv.Itoa(part))
	}
	versionString := strings.Join(stringParts, ".")
	result, err := version.NewVersion(versionString)
	if err != nil {
		panic(err)
	}
	return result
}

type versionList []*version.Version

func (list versionList) Len() int {
	return len(list)
}

func (list versionList) Less(i, j int) bool {
	return list[i].Compare(list[j]) < 0
}

func (list versionList) Swap(i, j int) {
	tmp := list[i]
	list[i] = list[j]
	list[j] = tmp
}
