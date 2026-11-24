// Package ui builds the icons desktop interface with NSGo.
package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
	"reacticons/internal/settings"
)

// Deps carries what the UI needs from main.
type Deps struct {
	DS   *dataset.Dataset
	Port int
}

// Main renders the whole app window content.
func Main(cx *core.Scope, deps *Deps) core.Node {
	ds := deps.DS

	// --- state ---
	querySig := core.NewSignal(cx, "")
	setSig := core.NewSignal(cx, "") // "" = all sets
	resultSig := core.NewSignalFunc(cx, ds.Search("", ""), nil)
	detailSig := core.NewSignal(cx, dataset.Icon{})
	detailOpenSig := core.NewSignal(cx, false)
	settingsOpenSig := core.NewSignal(cx, false)
	toaster := ui.NewToaster(cx)
	svgs := newSVGCache(4096)

	// --- search (in-memory index, ~ms: synchronous, no goroutine) ---
	runSearch := func() {
		resultSig.Set(ds.Search(querySig.Get(), setSig.Get()))
	}
	var stopDebounce func() bool
	onQueryChange := func(v string) {
		querySig.Set(v)
		if stopDebounce != nil {
			stopDebounce()
		}
		stopDebounce = cx.AfterFunc(300*time.Millisecond, runSearch)
	}
	onSubmitSearch := func() {
		if stopDebounce != nil {
			stopDebounce()
		}
		runSearch()
	}
	onSetPick := func(id string) {
		setSig.Set(id)
		runSearch()
	}

	// --- detail / settings helpers ---
	closeDetail := func() { detailOpenSig.Set(false) }
	cfg, _ := settings.Load() // fault-tolerant: defaults on any failure

	// Layout.
	return core.View{Class: appClass, Children: []core.Node{
		sidebar(cx, sidebarState{
			ds: ds, set: setSig, onPick: onSetPick,
		}),
		core.View{Class: "flex-1 flex-col min-w-0", Children: []core.Node{
			toolbar(cx, toolbarState{
				query: querySig, onChange: onQueryChange, onSubmit: onSubmitSearch,
				onSettings: func() { settingsOpenSig.Set(true) },
			}),
			iconGrid(cx, gridState{
				results: resultSig,
				onOpen: func(ic dataset.Icon) {
					detailSig.Set(ic)
					detailOpenSig.Set(true)
				},
				ds: ds, svgs: svgs,
			}),
			statusBar(cx, statusState{results: resultSig, version: ds.Version(), port: deps.Port}),
		}},
		core.Show{When: detailOpenSig.Get, Then: func(cx *core.Scope) core.Node {
			return detailPanel(cx, detailState{
				icon: detailSig, ds: ds, svgs: svgs, toaster: toaster,
				onClose: closeDetail,
			})
		}},
		core.Show{When: settingsOpenSig.Get, Then: func(cx *core.Scope) core.Node {
			return settingsPanel(cx, settingsState{
				st: cfg, ds: ds, toaster: toaster,
				onSaved: func(int) { settingsOpenSig.Set(false) },
			})
		}},
		ui.Toasts{Toaster: toaster},
	}}
}

const appClass = "w-full h-full flex-row bg-neutral-950 text-neutral-100 overflow-hidden"
