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
			n := len(s.results.Get())
			if n > maxGridRows {
				return fmt.Sprintf("共 %d 个图标 · 浏览前 %d 个，输入关键词查看其余结果 · v%s · MCP :%d", n, maxGridRows, s.version, s.port)
			}
			return fmt.Sprintf("共 %d 个图标 · v%s · MCP :%d", n, s.version, s.port)
		}},
	}}
}
