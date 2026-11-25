package importer

import (
	"encoding/json"
	"fmt"
	"sort"
)

// Data source identifiers stored in Set.Src.
const (
	SourceReactIcons = "react-icons"
	SourceIconify    = "iconify"
)

// IconifySet is a parsed Iconify collection JSON (iconify.design), reduced to
// the fields the pipeline needs.
type IconifySet struct {
	Prefix     string
	Name       string
	Total      int
	Author     string
	AuthorURL  string
	License    string
	LicenseURL string
	Width      int // set-level default, 0 when absent
	Height     int
	Icons      map[string]IconifyIcon
	Aliases    map[string]IconifyAlias
	Hidden     map[string]bool // top-level hidden names (rare)
}

// IconifyIcon is one icon entry: a body fragment plus optional per-icon
// dimensions and hidden flag.
type IconifyIcon struct {
	Body   string
	Width  int
	Height int
	Hidden bool
}

// IconifyAlias maps an alternate name to its parent icon, optionally
// overriding the body and dimensions.
type IconifyAlias struct {
	Parent string
	Body   string
	Width  int
	Height int
}

// IconifyEntry is a resolved, renderable icon: a unique name plus the body
// and dimensions to wrap it with.
type IconifyEntry struct {
	Name   string
	Body   string
	Width  int
	Height int
}

type iconifyFile struct {
	Prefix  string                      `json:"prefix"`
	Info    *iconifyInfo                `json:"info"`
	Width   *int                        `json:"width"`
	Height *int                        `json:"height"`
	Icons   map[string]iconifyIconJSON  `json:"icons"`
	Aliases map[string]iconifyAliasJSON `json:"aliases"`
	Hidden  map[string]bool             `json:"hidden"`
}

type iconifyInfo struct {
	Name    string `json:"name"`
	Total   int    `json:"total"`
	Author  struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"author"`
	License struct {
		Title string `json:"title"`
		URL   string `json:"url"`
	} `json:"license"`
}

type iconifyIconJSON struct {
	Body   string `json:"body"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
	Hidden bool   `json:"hidden"`
}

type iconifyAliasJSON struct {
	Parent string `json:"parent"`
	Body   string `json:"body"`
	Width  *int   `json:"width"`
	Height *int   `json:"height"`
}

// orZero dereferences an optional JSON number, mapping absent and
// non-positive values to 0 so callers can treat 0 as "unset".
func orZero(p *int) int {
	if p != nil && *p > 0 {
		return *p
	}
	return 0
}

// ParseIconifySet parses a raw Iconify collection JSON document.
func ParseIconifySet(b []byte) (IconifySet, error) {
	var f iconifyFile
	if err := json.Unmarshal(b, &f); err != nil {
		return IconifySet{}, fmt.Errorf("importer: parse iconify set: %w", err)
	}
	if f.Prefix == "" {
		return IconifySet{}, fmt.Errorf("importer: iconify set has no prefix")
	}
	s := IconifySet{
		Prefix:  f.Prefix,
		Width:   orZero(f.Width),
		Height:  orZero(f.Height),
		Icons:   make(map[string]IconifyIcon, len(f.Icons)),
		Aliases: make(map[string]IconifyAlias, len(f.Aliases)),
		Hidden:  f.Hidden,
	}
	if f.Info != nil {
		s.Name = f.Info.Name
		s.Total = f.Info.Total
		s.Author = f.Info.Author.Name
		s.AuthorURL = f.Info.Author.URL
		s.License = f.Info.License.Title
		s.LicenseURL = f.Info.License.URL
	}
	if s.Name == "" {
		s.Name = f.Prefix
	}
	for name, ic := range f.Icons {
		s.Icons[name] = IconifyIcon{
			Body:   ic.Body,
			Width:  orZero(ic.Width),
			Height: orZero(ic.Height),
			Hidden: ic.Hidden,
		}
	}
	for name, al := range f.Aliases {
		s.Aliases[name] = IconifyAlias{
			Parent: al.Parent,
			Body:   al.Body,
			Width:  orZero(al.Width),
			Height: orZero(al.Height),
		}
	}
	return s, nil
}

// ToSet converts the collection metadata into a manifest Set for cmd/prepare.
func (s IconifySet) ToSet() Set {
	return Set{
		ID:         s.Prefix,
		Name:       s.Name,
		ProjectURL: s.AuthorURL,
		License:    s.License,
		LicenseURL: s.LicenseURL,
		Src:        SourceIconify,
	}
}

// size resolves the effective dimensions for one icon: per-icon values first,
// then the set-level defaults, then 16x16 per the Iconify JSON spec, with a
// single known dimension implying a square icon.
func (s IconifySet) size(w, h int) (int, int) {
	if w <= 0 {
		w = s.Width
	}
	if h <= 0 {
		h = s.Height
	}
	if w <= 0 && h <= 0 {
		w, h = 16, 16
	}
	if w <= 0 {
		w = h
	}
	if h <= 0 {
		h = w
	}
	return w, h
}

// IconifyEntries resolves icons and aliases into renderable entries sorted by
// name. Hidden icons and entries whose alias chain cannot be resolved (or
// loops, guarded by a depth limit) are skipped.
func IconifyEntries(s IconifySet) []IconifyEntry {
	type resolved struct {
		body   string
		width  int
		height int
	}
	out := make([]IconifyEntry, 0, len(s.Icons)+len(s.Aliases))

	for name, ic := range s.Icons {
		if ic.Hidden || s.Hidden[name] || ic.Body == "" {
			continue
		}
		w, h := s.size(ic.Width, ic.Height)
		out = append(out, IconifyEntry{Name: name, Body: ic.Body, Width: w, Height: h})
	}

	for name, al := range s.Aliases {
		if s.Hidden[name] {
			continue
		}
		// Walk the alias chain: each hop may override body and dimensions.
		body, w, h := al.Body, al.Width, al.Height
		parent := al.Parent
		for depth := 0; depth < 5 && body == ""; depth++ {
			if ic, ok := s.Icons[parent]; ok && !ic.Hidden && ic.Body != "" {
				body = ic.Body
				if w <= 0 {
					w = ic.Width
				}
				if h <= 0 {
					h = ic.Height
				}
				break
			}
			next, ok := s.Aliases[parent]
			if !ok {
				break
			}
			parent = next.Parent
		}
		if body == "" {
			continue
		}
		w, h = s.size(w, h)
		out = append(out, IconifyEntry{Name: name, Body: body, Width: w, Height: h})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IconifySVG wraps an icon body fragment into a complete SVG document with an
// explicit viewBox, the format the bundled dataset stores. No fill is baked
// in: monochrome bodies carry currentColor themselves and colored bodies keep
// their palette, matching how the UI and renderer tint icons.
func IconifySVG(body string, w, h int) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">%s</svg>`, w, h, body)
}
