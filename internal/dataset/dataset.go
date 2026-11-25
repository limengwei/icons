// Package dataset holds access to the bundled react-icons dataset.
//
// The index (sets + icons) is decompressed into memory at Open time;
// per-set SVG payloads are lazily decompressed and cached on first use.
package dataset

import (
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

//go:embed data/index.json.gz
//go:embed data/svg/*.json.gz
var dataFS embed.FS

// SetInfo describes one icon set (e.g. fa6 "Font Awesome 6").
type SetInfo struct {
	ID         string
	Name       string
	Count      int
	ProjectURL string
	License    string
	LicenseURL string
	Source     string // "react-icons" or "iconify"
}

// SourceInfo describes one data source bundled into the dataset.
type SourceInfo struct {
	Name    string // "react-icons" or "iconify"
	Version string // package version / fetch date
}

// Icon is one searchable icon entry.
type Icon struct {
	Set        string
	Name       string
	SearchText string
}

// Dataset is the in-memory icon index. It is safe for concurrent use.
type Dataset struct {
	version string
	sources []SourceInfo
	sets    []SetInfo
	setByID map[string]SetInfo
	icons   []Icon

	mu   sync.Mutex
	svgs map[string]map[string]string // set id -> icon name -> svg
}

type indexFile struct {
	Version string        `json:"version"`
	Sources []indexSource `json:"sources"`
	Sets    []indexSet    `json:"sets"`
	Icons   []indexIcon   `json:"icons"`
}

type indexSource struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type indexSet struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ProjectURL string `json:"projectUrl"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	Src        string `json:"src,omitempty"`
}

type indexIcon struct {
	S string `json:"s"`
	N string `json:"n"`
	W string `json:"w"`
}

// Open decompresses and parses the bundled index into memory.
func Open() (*Dataset, error) {
	raw, err := readGzip("data/index.json.gz")
	if err != nil {
		return nil, fmt.Errorf("dataset: read index: %w", err)
	}
	var idx indexFile
	if err := json.Unmarshal(raw, &idx); err != nil {
		return nil, fmt.Errorf("dataset: parse index: %w", err)
	}

	d := &Dataset{
		version: idx.Version,
		setByID: make(map[string]SetInfo, len(idx.Sets)),
		svgs:    make(map[string]map[string]string),
	}

	d.sets = make([]SetInfo, len(idx.Sets))
	counts := make(map[string]int, len(idx.Sets))
	for i, s := range idx.Sets {
		d.sets[i] = SetInfo{
			ID:         s.ID,
			Name:       s.Name,
			ProjectURL: s.ProjectURL,
			License:    s.License,
			LicenseURL: s.LicenseURL,
			Source:     s.Src,
		}
	}
	if len(idx.Sources) > 0 {
		d.sources = make([]SourceInfo, len(idx.Sources))
		for i, s := range idx.Sources {
			d.sources[i] = SourceInfo{Name: s.Name, Version: s.Version}
		}
	} else {
		d.sources = []SourceInfo{{Name: "react-icons", Version: idx.Version}}
	}

	d.icons = make([]Icon, len(idx.Icons))
	for i, ic := range idx.Icons {
		d.icons[i] = Icon{
			Set:        ic.S,
			Name:       ic.N,
			SearchText: strings.ToLower(ic.W),
		}
		counts[ic.S]++
	}

	for i := range d.sets {
		d.sets[i].Count = counts[d.sets[i].ID]
		d.setByID[d.sets[i].ID] = d.sets[i]
	}

	return d, nil
}

// Version returns the react-icons version of the bundled data.
func (d *Dataset) Version() string { return d.version }

// Sources returns the metadata of every bundled data source, in dataset
// order (react-icons first).
func (d *Dataset) Sources() []SourceInfo { return d.sources }

// Sets returns all sets in manifest order.
func (d *Dataset) Sets() []SetInfo { return d.sets }

// Set returns the SetInfo for the given set id.
func (d *Dataset) Set(id string) (SetInfo, bool) {
	s, ok := d.setByID[id]
	return s, ok
}

// Total returns the total number of icons.
func (d *Dataset) Total() int { return len(d.icons) }

// Search filters icons by a substring of their search text and optionally by
// set. When both query and setID are empty it returns the internal icon slice
// itself (not a copy); callers must treat it as read-only.
func (d *Dataset) Search(query, setID string) []Icon {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" && setID == "" {
		return d.icons
	}
	var res []Icon
	for i := range d.icons {
		ic := &d.icons[i]
		if setID != "" && ic.Set != setID {
			continue
		}
		if query != "" && !strings.Contains(ic.SearchText, query) {
			continue
		}
		res = append(res, *ic)
	}
	return res
}

// GetSVG returns the raw SVG markup for an icon, lazily decompressing and
// caching the owning set's payload on first access.
func (d *Dataset) GetSVG(set, name string) (string, error) {
	m, err := d.svgMap(set)
	if err != nil {
		return "", err
	}
	svg, ok := m[name]
	if !ok {
		return "", fmt.Errorf("dataset: icon %q not found in set %q", name, set)
	}
	return svg, nil
}

func (d *Dataset) svgMap(set string) (map[string]string, error) {
	d.mu.Lock()
	if m, ok := d.svgs[set]; ok {
		d.mu.Unlock()
		return m, nil
	}
	d.mu.Unlock()

	raw, err := readGzip("data/svg/" + set + ".json.gz")
	if err != nil {
		return nil, fmt.Errorf("dataset: load set %q: %w", set, err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("dataset: parse set %q: %w", set, err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if existing, ok := d.svgs[set]; ok { // another goroutine won the race
		return existing, nil
	}
	d.svgs[set] = m
	return m, nil
}

func readGzip(name string) ([]byte, error) {
	f, err := dataFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
