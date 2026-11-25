// Command fetch-iconify downloads curated Iconify collections and converts
// them into the per-icon SVG layout that cmd/prepare ingests.
//
// For each prefix it fetches the collection JSON from the iconify/icon-sets
// repository, writes one SVG per icon under
//
//	data/iconify/{prefix}/{name}.svg
//
// plus a manifest with set metadata and the fetch date:
//
//	data/iconify/manifest.json   {"fetched": "...", "sets": [...]}
//
// Every generated SVG is validated with oksvg (the same parser the app's PNG
// renderer uses); failures are reported but do not stop the run, so bad sets
// can be identified and dropped from the curated list.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/srwiley/oksvg"

	"reacticons/internal/importer"
)

// curatedSets lists Iconify collections that react-icons does not carry.
// Order is preserved in the final dataset.
var curatedSets = []string{
	"solar",
	"hugeicons",
	"iconoir",
	"iconamoon",
	"lets-icons",
	"maki",
	"fluent-color",
}

const defaultBaseURL = "https://raw.githubusercontent.com/iconify/icon-sets/master/json"

func main() {
	setsFlag := flag.String("sets", "", "comma-separated Iconify prefixes (default: curated list)")
	outDir := flag.String("out", "data/iconify", "output directory")
	baseURL := flag.String("base", defaultBaseURL, "base URL to fetch <prefix>.json from")
	flag.Parse()

	prefixes := curatedSets
	if *setsFlag != "" {
		prefixes = strings.Split(*setsFlag, ",")
	}

	if err := run(prefixes, *outDir, *baseURL); err != nil {
		fmt.Fprintln(os.Stderr, "fetch-iconify:", err)
		os.Exit(1)
	}
}

type manifestDoc struct {
	Fetched string         `json:"fetched"`
	Sets    []importer.Set `json:"sets"`
}

func run(prefixes []string, outDir, baseURL string) error {
	manifest := manifestDoc{Fetched: time.Now().Format("2006-01-02")}
	totalIcons, badTotal := 0, 0

	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		set, err := fetchSet(baseURL, prefix)
		if err != nil {
			return err
		}
		entries := importer.IconifyEntries(set)
		dir := filepath.Join(outDir, set.Prefix)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}

		bad := 0
		for _, e := range entries {
			svg := importer.IconifySVG(e.Body, e.Width, e.Height)
			name := filepath.Join(dir, e.Name+".svg")
			if err := os.WriteFile(name, []byte(svg), 0o644); err != nil {
				return err
			}
			// Validate the way render.Rasterize will consume it: oksvg cannot
			// parse the literal "currentColor", which the renderer resolves
			// to a concrete color first.
			probe := strings.ReplaceAll(svg, "currentColor", "#000000")
			if _, err := oksvg.ReadIconStream(strings.NewReader(probe)); err != nil {
				bad++
				if bad <= 5 {
					fmt.Printf("  ! %s/%s: oksvg: %v\n", set.Prefix, e.Name, err)
				}
			}
		}
		badTotal += bad
		totalIcons += len(entries)
		manifest.Sets = append(manifest.Sets, set.ToSet())
		fmt.Printf("%-14s %5d icons  %s (%s)\n", set.Prefix, len(entries), set.Name, set.License)
		if set.Total > 0 && set.Total != len(entries) {
			fmt.Printf("  note: upstream total %d differs from %d entries (hidden/unresolvable skipped)\n", set.Total, len(entries))
		}
		if bad > 5 {
			fmt.Printf("  ! %d more icons failed oksvg validation\n", bad-5)
		}
	}

	if len(manifest.Sets) == 0 {
		return fmt.Errorf("no collections fetched")
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}

	fmt.Printf("\niconify %s: %d sets, %d icons, %d failed oksvg validation\n",
		manifest.Fetched, len(manifest.Sets), totalIcons, badTotal)
	fmt.Printf("output       : %s\n", outDir)
	return nil
}

func fetchSet(baseURL, prefix string) (importer.IconifySet, error) {
	url := baseURL + "/" + prefix + ".json"
	resp, err := http.Get(url)
	if err != nil {
		return importer.IconifySet{}, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return importer.IconifySet{}, fmt.Errorf("fetch %s: status %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return importer.IconifySet{}, fmt.Errorf("read %s: %w", url, err)
	}
	set, err := importer.ParseIconifySet(b)
	if err != nil {
		return importer.IconifySet{}, fmt.Errorf("parse %s: %w", url, err)
	}
	if set.Prefix != prefix {
		return importer.IconifySet{}, fmt.Errorf("parse %s: prefix mismatch: %q", url, set.Prefix)
	}
	return set, nil
}
