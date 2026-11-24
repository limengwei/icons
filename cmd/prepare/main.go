// Command prepare builds the bundled dataset under internal/dataset/data
// from the extracted react-icons sources under data/.
//
// Inputs:
//
//	data/react-icons/react-icons/package/lib/iconsManifest.js   set metadata
//	data/react-icons/react-icons/package/package.json           version
//	data/package/{set}/{IconName}.svg                           extracted icons
//
// Outputs:
//
//	internal/dataset/data/index.json.gz       version + sets + search index
//	internal/dataset/data/svg/{set}.json.gz   per-set {"Name":"<svg ...>"}
//
// Icons appear in manifest set order, sorted by file name within each set.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"reacticons/internal/importer"
)

const (
	manifestPath = "data/react-icons/react-icons/package/lib/iconsManifest.js"
	pkgPath      = "data/react-icons/react-icons/package/package.json"
	svgSrcDir    = "data/package"
	outDir       = "internal/dataset/data"
)

type indexDoc struct {
	Version string         `json:"version"`
	Sets    []importer.Set `json:"sets"`
	Icons   []indexIcon    `json:"icons"`
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
	sets, err := readSets()
	if err != nil {
		return err
	}
	version, err := readVersion()
	if err != nil {
		return err
	}
	svgDir := filepath.Join(outDir, "svg")
	if err := os.MkdirAll(svgDir, 0o755); err != nil {
		return err
	}

	doc := indexDoc{Version: version, Sets: sets}
	total := 0
	for _, set := range sets {
		names, err := listIconNames(set.ID)
		if err != nil {
			return err
		}
		svgs := make(map[string]string, len(names))
		for _, name := range names {
			b, err := os.ReadFile(filepath.Join(svgSrcDir, set.ID, name+".svg"))
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
		fmt.Printf("%-5s %5d icons\n", set.ID, len(names))
	}

	idxSize, err := writeGzipJSON(filepath.Join(outDir, "index.json.gz"), doc)
	if err != nil {
		return err
	}
	files, svgBytes := dirStats(svgDir)

	fmt.Printf("\nreact-icons %s: %d sets, %d icons\n", version, len(sets), total)
	fmt.Printf("index.json.gz : %s\n", humanSize(idxSize))
	fmt.Printf("svg/          : %d files, %s total\n", files, humanSize(svgBytes))
	fmt.Printf("output        : %s\n", outDir)
	return nil
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
// one set directory.
func listIconNames(set string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(svgSrcDir, set))
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
		return nil, fmt.Errorf("set %s: no .svg files in %s", set, filepath.Join(svgSrcDir, set))
	}
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
