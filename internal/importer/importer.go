// Package importer ingests the react-icons source tree into the local dataset.
//
// It provides the pure helpers shared by the cmd/prepare tool: parsing the
// iconsManifest.js manifest, splitting camelCase icon names into search words
// and building searchable text for a single icon.
package importer

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Set describes one icon collection from the react-icons manifest.
type Set struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProjectURL string `json:"projectUrl"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
}

// ParseManifest parses the contents of an iconsManifest.js file and returns
// its icon sets in manifest order, skipping the non-icon "lib" entry.
func ParseManifest(src string) ([]Set, error) {
	const marker = "IconsManifest"
	k := strings.Index(src, marker)
	if k < 0 {
		return nil, fmt.Errorf("importer: %s not found", marker)
	}
	rest := src[k+len(marker):]
	start := strings.IndexByte(rest, '[')
	if start < 0 {
		return nil, fmt.Errorf("importer: no '[' after %s", marker)
	}
	end, ok := matchBracket(rest, start)
	if !ok {
		return nil, fmt.Errorf("importer: unbalanced brackets after %s", marker)
	}
	var all []Set
	if err := json.Unmarshal([]byte(rest[start:end+1]), &all); err != nil {
		return nil, fmt.Errorf("importer: bad JSON in %s: %w", marker, err)
	}
	sets := make([]Set, 0, len(all))
	for _, s := range all {
		if s.ID == "lib" {
			continue
		}
		sets = append(sets, s)
	}
	return sets, nil
}

// matchBracket returns the index of the ']' matching the '[' at start,
// skipping brackets that appear inside double-quoted strings.
func matchBracket(s string, start int) (int, bool) {
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case inStr && esc:
			esc = false
		case inStr && c == '\\':
			esc = true
		case inStr && c == '"':
			inStr = false
		case c == '"':
			inStr = true
		case c == '[':
			depth++
		case c == ']':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

// SplitName splits a camelCase icon name into lowercase words: a split goes
// before each uppercase letter (keeping acronym runs such as "CSS" together)
// and at letter-to-digit boundaries, while digits stay attached to the
// letters that follow them ("Fa500px" -> "fa 500px").
func SplitName(name string) string {
	var b strings.Builder
	b.Grow(len(name) + 8)
	for i := 0; i < len(name); i++ {
		if i > 0 && splitBefore(name, i) {
			b.WriteByte(' ')
		}
		b.WriteByte(lower(name[i]))
	}
	return b.String()
}

// splitBefore reports whether name[i] starts a new word.
func splitBefore(name string, i int) bool {
	c, prev := name[i], name[i-1]
	switch {
	case isUpper(c):
		if isLower(prev) || isDigit(prev) {
			return true
		}
		// End of an acronym run, as in the "S" of "HTTPSClient".
		return isUpper(prev) && i+1 < len(name) && isLower(name[i+1])
	case isDigit(c) && isLetter(prev):
		return true
	}
	return false
}

// SearchText builds the search text for one icon: the lowercased original
// name, its SplitName words, the set ID and the words of the set name (split
// on non-alphanumeric characters), deduplicated and space-joined.
func SearchText(name, setID, setName string) string {
	var tokens []string
	seen := make(map[string]bool)
	add := func(t string) {
		if t == "" || seen[t] {
			return
		}
		seen[t] = true
		tokens = append(tokens, t)
	}
	add(strings.ToLower(name))
	for _, w := range strings.Fields(SplitName(name)) {
		add(w)
	}
	add(strings.ToLower(setID))
	for _, w := range strings.FieldsFunc(setName, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		add(strings.ToLower(w))
	}
	return strings.Join(tokens, " ")
}

func isLetter(c byte) bool { return isLower(c) || isUpper(c) }
func isLower(c byte) bool  { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool  { return c >= 'A' && c <= 'Z' }
func isDigit(c byte) bool  { return c >= '0' && c <= '9' }
func lower(c byte) byte {
	if isUpper(c) {
		return c + 'a' - 'A'
	}
	return c
}
