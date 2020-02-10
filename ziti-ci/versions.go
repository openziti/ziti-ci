package main

import (
	"github.com/hashicorp/go-version"
	"strconv"
	"strings"
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
