package ui

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"
)

// toolbarState is the top bar's inputs.
type toolbarState struct {
	query      *core.Signal[string]
	onChange   func(string)
	onSubmit   func()
	onSettings func()
}

// toolbar renders the search box and action buttons.
func toolbar(cx *core.Scope, s toolbarState) core.Node {
	return core.View{Class: "p-3 gap-2 flex-row items-center", Children: []core.Node{
		core.View{Class: "flex-1 min-w-0", Children: []core.Node{
			ui.Input{
				Value:       s.query.Get,
				OnChange:    s.onChange,
				OnSubmit:    func(string) { s.onSubmit() },
				Placeholder: "搜索图标（如 beer / youtube）…",
				AutoFocus:   true,
			},
		}},
		ui.Button{Text: "设置", OnPress: s.onSettings},
	}}
}
