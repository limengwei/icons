package dataset

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func mustOpen(t *testing.T) *Dataset {
	t.Helper()
	start := time.Now()
	d, err := Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Logf("Open took %s", time.Since(start))
	return d
}

func TestOpen(t *testing.T) {
	d := mustOpen(t)
	if got := d.Version(); got != "5.7.0" {
		t.Errorf("Version() = %q, want %q", got, "5.7.0")
	}
	if got := len(d.Sets()); got != 31 {
		t.Errorf("len(Sets()) = %d, want 31", got)
	}
	if got := d.Total(); got != 50934 {
		t.Errorf("Total() = %d, want 50934", got)
	}
	s, ok := d.Set("fa6")
	if !ok {
		t.Fatal(`Set("fa6") not found`)
	}
	if s.Count != 2058 {
		t.Errorf("fa6 Count = %d, want 2058", s.Count)
	}
	if _, ok := d.Set("nope"); ok {
		t.Error(`Set("nope") reported found`)
	}
}

func TestSearch(t *testing.T) {
	d := mustOpen(t)

	if got := len(d.Search("", "")); got != d.Total() {
		t.Errorf(`Search("","") len = %d, want %d`, got, d.Total())
	}

	if !containsIcon(d, "beer", "fa6", "FaBeerMugEmpty") {
		t.Error(`Search("beer","") missing fa6 FaBeerMugEmpty`)
	}
	if !containsIcon(d, "youtube", "", "FaYoutube") {
		t.Error(`Search("youtube","") missing FaYoutube`)
	}

	fa6 := d.Search("", "fa6")
	if len(fa6) != 2058 {
		t.Errorf(`Search("","fa6") len = %d, want 2058`, len(fa6))
	}
	for _, ic := range fa6 {
		if ic.Set != "fa6" {
			t.Fatalf(`Search("","fa6") returned icon from set %q`, ic.Set)
		}
	}

	if got := len(d.Search("font awesome", "")); got == 0 {
		t.Error(`Search("font awesome","") returned no results`)
	}

	if got := len(d.Search("  BEER ", "")); got != len(d.Search("beer", "")) {
		t.Error("Search is not trimming/lowercasing the query")
	}
}

func containsIcon(d *Dataset, query, set, name string) bool {
	for _, ic := range d.Search(query, set) {
		if ic.Name == name {
			return true
		}
	}
	return false
}

func TestGetSVG(t *testing.T) {
	d := mustOpen(t)

	svg, err := d.GetSVG("fa6", "FaBeerMugEmpty")
	if err != nil {
		t.Fatalf(`GetSVG("fa6","FaBeerMugEmpty") error = %v`, err)
	}
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Errorf("GetSVG fa6 returned malformed SVG: %.80s", svg)
	}

	if _, err := d.GetSVG("nope", "Whatever"); err == nil {
		t.Error(`GetSVG("nope",...) expected error for missing set`)
	}
	if _, err := d.GetSVG("fa6", "NoSuchIcon"); err == nil {
		t.Error(`GetSVG("fa6","NoSuchIcon") expected error for missing icon`)
	}

	if _, err := d.GetSVG("go", "GoZap"); err != nil { // second set: lazy load
		t.Fatalf(`GetSVG("go","GoZap") error = %v`, err)
	}
}

func TestConcurrent(t *testing.T) {
	d := mustOpen(t)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				d.Search("arrow", "")
				d.Search("", "fa6")
				if _, err := d.GetSVG("fa6", "FaBeerMugEmpty"); err != nil {
					t.Errorf("GetSVG fa6: %v", err)
					return
				}
				if _, err := d.GetSVG("go", "GoZap"); err != nil {
					t.Errorf("GetSVG go: %v", err)
					return
				}
				_ = d.Version()
			}
		}(g)
	}
	wg.Wait()
}
