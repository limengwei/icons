package importer

import (
	"reflect"
	"strings"
	"testing"
)

const sampleIconifySet = `{
  "prefix": "solar",
  "info": {
    "name": "Solar",
    "total": 4,
    "author": {"name": "480 Design", "url": "https://example.com/solar"},
    "license": {"title": "CC BY 4.0", "spdx": "CC-BY-4.0", "url": "https://creativecommons.org/licenses/by/4.0/"}
  },
  "width": 24,
  "height": 24,
  "icons": {
    "home": {"body": "<path d='home'/>"},
    "wide": {"body": "<path d='wide'/>", "width": 48},
    "gone": {"hidden": true},
    "empty": {}
  },
  "aliases": {
    "house": {"parent": "home"},
    "alias-of-alias": {"parent": "house"},
    "dead-alias": {"parent": "missing"},
    "loop-a": {"parent": "loop-b"},
    "loop-b": {"parent": "loop-a"},
    "wide2": {"parent": "wide", "height": 12}
  },
  "hidden": {"home-hidden": true}
}`

func TestParseIconifySet(t *testing.T) {
	s, err := ParseIconifySet([]byte(sampleIconifySet))
	if err != nil {
		t.Fatalf("ParseIconifySet: %v", err)
	}
	if s.Prefix != "solar" || s.Name != "Solar" || s.Total != 4 {
		t.Errorf("got %+v", s)
	}
	if s.License != "CC BY 4.0" || s.LicenseURL != "https://creativecommons.org/licenses/by/4.0/" {
		t.Errorf("license = %q %q", s.License, s.LicenseURL)
	}
	if s.Author != "480 Design" || s.AuthorURL != "https://example.com/solar" {
		t.Errorf("author = %q %q", s.Author, s.AuthorURL)
	}
	if s.Width != 24 || s.Height != 24 {
		t.Errorf("dims = %dx%d, want 24x24", s.Width, s.Height)
	}
	if len(s.Icons) != 4 || len(s.Aliases) != 6 {
		t.Errorf("icons = %d, aliases = %d", len(s.Icons), len(s.Aliases))
	}
}

func TestParseIconifySetErrors(t *testing.T) {
	if _, err := ParseIconifySet([]byte(`{"prefix":"","icons":{}}`)); err == nil {
		t.Error("empty prefix accepted")
	}
	if _, err := ParseIconifySet([]byte(`not json`)); err == nil {
		t.Error("bad json accepted")
	}
}

func TestIconifyEntries(t *testing.T) {
	s, err := ParseIconifySet([]byte(sampleIconifySet))
	if err != nil {
		t.Fatalf("ParseIconifySet: %v", err)
	}
	got := IconifyEntries(s)
	want := []IconifyEntry{
		{Name: "alias-of-alias", Body: "<path d='home'/>", Width: 24, Height: 24},
		{Name: "home", Body: "<path d='home'/>", Width: 24, Height: 24},
		{Name: "house", Body: "<path d='home'/>", Width: 24, Height: 24},
		{Name: "wide", Body: "<path d='wide'/>", Width: 48, Height: 24},
		{Name: "wide2", Body: "<path d='wide'/>", Width: 48, Height: 12},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entries = %+v\nwant    %+v", got, want)
	}
}

func TestIconifySetDefaultsSize(t *testing.T) {
	// No set-level dimensions: default 16x16; single icon dimension implies
	// a square; alias overrides win over the parent.
	s := IconifySet{Icons: map[string]IconifyIcon{"a": {Body: "x"}}, Aliases: map[string]IconifyAlias{"b": {Parent: "a"}}}
	got := IconifyEntries(s)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	for _, e := range got {
		if e.Width != 16 || e.Height != 16 {
			t.Errorf("%s dims = %dx%d, want 16x16", e.Name, e.Width, e.Height)
		}
	}

	s2 := IconifySet{Height: 20, Icons: map[string]IconifyIcon{"a": {Body: "x"}}}
	if e := IconifyEntries(s2)[0]; e.Width != 20 || e.Height != 20 {
		t.Errorf("height-only set: dims = %dx%d, want 20x20", e.Width, e.Height)
	}
}

func TestIconifySVG(t *testing.T) {
	got := IconifySVG("<path d='x'/>", 24, 16)
	want := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 16"><path d='x'/></svg>`
	if got != want {
		t.Errorf("IconifySVG = %q, want %q", got, want)
	}
}

func TestIconifySetToSet(t *testing.T) {
	s := IconifySet{
		Prefix: "solar", Name: "Solar",
		AuthorURL: "https://example.com/solar",
		License:   "CC BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
	}
	got := s.ToSet()
	want := Set{
		ID: "solar", Name: "Solar",
		ProjectURL: "https://example.com/solar",
		License:    "CC BY 4.0", LicenseURL: "https://creativecommons.org/licenses/by/4.0/",
		Src:        SourceIconify,
	}
	if got != want {
		t.Errorf("ToSet = %+v, want %+v", got, want)
	}
}

func TestSearchTextKebab(t *testing.T) {
	got := SearchText("home-minimal", "solar", "Solar")
	// Words: "home-minimal" (raw name) "home" "minimal" "solar".
	for _, w := range []string{"home-minimal", "home", "minimal", "solar"} {
		if !strings.Contains(" "+got+" ", " "+w+" ") {
			t.Errorf("SearchText(%q) = %q, missing word %q", "home-minimal", got, w)
		}
	}
}
