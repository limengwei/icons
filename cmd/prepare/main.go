// Command prepare builds the bundled dataset under internal/dataset/data
// from the extracted react-icons sources and the fetched Iconify collections
// under data/.
//
// Inputs:
//
//	data/react-icons/react-icons/package/lib/iconsManifest.js   set metadata
//	data/react-icons/react-icons/package/package.json           version
//	data/package/{set}/{IconName}.svg                           react-icons SVGs
//	data/iconify/manifest.json                                  iconify set metadata + fetch date
//	data/iconify/{prefix}/{name}.svg                            iconify SVGs
//
// Outputs:
//
//	internal/dataset/data/index.json.gz       sources + sets + search index
//	internal/dataset/data/svg/{set}.json.gz   per-set {"Name":"<svg ...>"}
//
// Icons appear in manifest set order (react-icons first, then iconify),
// sorted by file name within each set.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"reacticons/internal/importer"
)

const (
	manifestPath   = "data/react-icons/react-icons/package/lib/iconsManifest.js"
	pkgPath        = "data/react-icons/react-icons/package/package.json"
	svgSrcDir      = "data/package"
	iconifyDir     = "data/iconify"
	iconifyManPath = iconifyDir + "/manifest.json"
	outDir         = "internal/dataset/data"
)

type sourceInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type indexDoc struct {
	Version string         `json:"version"` // react-icons package version
	Sources []sourceInfo   `json:"sources"`
	Sets    []importer.Set `json:"sets"`
	Icons   []indexIcon    `json:"icons"`
}

type iconifyManifest struct {
	Fetched string         `json:"fetched"`
	Sets    []importer.Set `json:"sets"`
}

type indexIcon struct {
	S string `json:"s"` // set id
	N string `json:"n"` // icon name
	W string `json:"w"` // search text
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "prepare:", err)
		os.Exit(1)
	}
}

func run() error {
	// The react-icons npm tree is optional: when it has been cleaned up
	// after extraction, fall back to the previously built index for set
	// metadata and the version.
	var version string
	var riSets []importer.Set
	if _, err := os.Stat(manifestPath); err == nil {
		version, err = readVersion()
		if err != nil {
			return err
		}
		riSets, err = readSets()
		if err != nil {
			return err
		}
	} else {
		fmt.Println("note: no react-icons npm tree at", manifestPath, "— reusing previous index metadata")
		version, riSets, err = readPreviousIndex()
		if err != nil {
			return err
		}
	}
	sets, sources, err := mergeSources(version, riSets)
	if err != nil {
		return err
	}
	svgDir := filepath.Join(outDir, "svg")
	if err := os.MkdirAll(svgDir, 0o755); err != nil {
		return err
	}

	doc := indexDoc{Version: version, Sources: sources, Sets: sets}
	total := 0
	for _, set := range sets {
		names, err := listIconNames(svgDirFor(set), set.ID)
		if err != nil {
			return err
		}
		svgs := make(map[string]string, len(names))
		for _, name := range names {
			b, err := os.ReadFile(filepath.Join(svgDirFor(set), set.ID, name+".svg"))
			if err != nil {
				return err
			}
			svgs[name] = string(b)
			doc.Icons = append(doc.Icons, indexIcon{
				S: set.ID,
				N: name,
				W: importer.SearchText(name, set.ID, set.Name),
			})
		}
		if _, err := writeGzipJSON(filepath.Join(svgDir, set.ID+".json.gz"), svgs); err != nil {
			return err
		}
		total += len(names)
		fmt.Printf("%-14s %5d icons\n", set.ID, len(names))
	}

	idxSize, err := writeGzipJSON(filepath.Join(outDir, "index.json.gz"), doc)
	if err != nil {
		return err
	}
	files, svgBytes := dirStats(svgDir)

	for _, src := range sources {
		fmt.Printf("\n%s %s\n", src.Name, src.Version)
	}
	fmt.Printf("total: %d sets, %d icons\n", len(sets), total)
	fmt.Printf("index.json.gz : %s\n", humanSize(idxSize))
	fmt.Printf("svg/          : %d files, %s total\n", files, humanSize(svgBytes))
	fmt.Printf("output        : %s\n", outDir)
	return nil
}

// svgDirFor returns the source directory holding the per-icon SVG files of a
// set, based on its source identifier.
func svgDirFor(set importer.Set) string {
	if set.Src == importer.SourceIconify {
		return iconifyDir
	}
	return svgSrcDir
}

// readPreviousIndex recovers the react-icons set metadata and version from
// the last built index.json.gz, for when the raw npm tree is gone but the
// extracted per-icon SVGs under data/package are still around.
func readPreviousIndex() (string, []importer.Set, error) {
	b, err := readGzipFile(filepath.Join(outDir, "index.json.gz"))
	if err != nil {
		return "", nil, fmt.Errorf("no react-icons manifest and no previous index: %w", err)
	}
	var doc indexDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return "", nil, fmt.Errorf("parse previous index: %w", err)
	}
	sets := make([]importer.Set, 0, len(doc.Sets))
	for _, s := range doc.Sets {
		if s.Src == "" || s.Src == importer.SourceReactIcons {
			sets = append(sets, importer.Set{
				ID: s.ID, Name: s.Name, ProjectURL: s.ProjectURL,
				License: s.License, LicenseURL: s.LicenseURL,
				Src: importer.SourceReactIcons,
			})
		}
	}
	if doc.Version == "" || len(sets) == 0 {
		return "", nil, fmt.Errorf("previous index has no react-icons data")
	}
	return doc.Version, sets, nil
}

// readGzipFile decompresses a gzip file.
func readGzipFile(path string) ([]byte, error) {
	f, err := os.Open(path)
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

// mergeSources merges the react-icons sets with the iconify manifest
// (react-icons first) and builds the source metadata list. Iconify prefixes
// that collide with an existing set id are skipped with a warning.
func mergeSources(riVersion string, sets []importer.Set) ([]importer.Set, []sourceInfo, error) {
	sources := []sourceInfo{{Name: importer.SourceReactIcons, Version: riVersion}}
	seen := make(map[string]bool, len(sets))
	for _, s := range sets {
		seen[s.ID] = true
	}

	if _, err := os.Stat(iconifyManPath); err == nil {
		b, err := os.ReadFile(iconifyManPath)
		if err != nil {
			return nil, nil, err
		}
		var m iconifyManifest
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", iconifyManPath, err)
		}
		added := 0
		for _, s := range m.Sets {
			if s.Src == "" {
				s.Src = importer.SourceIconify
			}
			if seen[s.ID] {
				fmt.Fprintf(os.Stderr, "warning: iconify set %s collides with existing set id, skipped\n", s.ID)
				continue
			}
			seen[s.ID] = true
			sets = append(sets, s)
			added++
		}
		if added > 0 {
			ver := m.Fetched
			if ver == "" {
				ver = "unknown"
			}
			sources = append(sources, sourceInfo{Name: importer.SourceIconify, Version: ver})
		}
	} else {
		fmt.Println("note: no iconify manifest at", iconifyManPath, "— building react-icons only")
	}
	return sets, sources, nil
}

// readSets parses the icon manifest, keeping manifest order.
func readSets() ([]importer.Set, error) {
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	return importer.ParseManifest(string(b))
}

// readVersion returns the version from the react-icons package.json.
func readVersion() (string, error) {
	b, err := os.ReadFile(pkgPath)
	if err != nil {
		return "", err
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(b, &pkg); err != nil {
		return "", fmt.Errorf("parse %s: %w", pkgPath, err)
	}
	return pkg.Version, nil
}

// listIconNames returns the sorted icon names (file names without .svg) of
// one set directory under dir.
func listIconNames(dir, set string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, set))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".svg"))
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("set %s: no .svg files in %s", set, filepath.Join(dir, set))
	}
	sort.Strings(names)
	return names, nil
}

// writeGzipJSON marshals v, gzip-compresses it at the best compression level
// and writes it to path, returning the compressed size.
func writeGzipJSON(path string, v any) (int64, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return 0, err
	}
	if _, err := zw.Write(data); err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return 0, err
	}
	return int64(buf.Len()), nil
}

// dirStats returns the file count and total byte size of a directory.
func dirStats(dir string) (int, int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	files, total := 0, int64(0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files++
		total += info.Size()
	}
	return files, total
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.2f MiB (%d bytes)", float64(n)/(1<<20), n)
	case n >= 1<<10:
		return fmt.Sprintf("%.2f KiB (%d bytes)", float64(n)/(1<<10), n)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}
