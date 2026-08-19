package lint

import (
	"sort"
	"strings"
)

// Line returns the one-based line number an offset falls on.
//
// An address is what turns a report into an edit, so every rule that can name
// a line does. This lives here rather than in one linter because all three
// need it and a second copy is a second chance to get the off-by-one wrong.
func Line(body string, index int) int {
	if index > len(body) {
		index = len(body)
	}
	return strings.Count(body[:index], "\n") + 1
}

// At builds the "path:line" address a finding carries.
func At(path, body string, index int) string {
	return path + ":" + itoa(Line(body, index))
}

// IsWordByte reports whether a byte continues a word, which is the class a
// regular expression writes as \w. The rules that replaced a lookaround with a
// check beside the match all need this same boundary.
func IsWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// SortedKeys returns a map's keys in order, so a run reports the same findings
// in the same order twice.
func SortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// First returns at most n of the strings given, for a message that names a few
// examples rather than every one.
func First(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// Truncate cuts a string to a length that fits a terminal line.
func Truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
