package ui

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/core"

	"reacticons/internal/dataset"
)

// statusState feeds the bottom status bar.
type statusState struct {
	results *core.Signal[[]dataset.Icon]
	version string
	port    int
}

// statusBar shows the current result count, dataset version and MCP port.
func statusBar(cx *core.Scope, s statusState) core.Node {
	return core.View{Class: "px-3 py-2 border-t border-neutral-800 text-xs text-neutral-500", Children: []core.Node{
		core.Text{ContentFunc: func() string {
			return fmt.Sprintf("共 %d 个图标 · v%s · MCP :%d", len(s.results.Get()), s.version, s.port)
		}},
	}}
}
