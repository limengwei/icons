package ui

import (
	"fmt"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
)

// repoURL is the project's GitHub repository, opened in the system browser.
const repoURL = "https://github.com/limengwei/icons"

// githubSVG is GitHub's octocat mark, taken from the app's own dataset
// (si/SiGithub, Simple Icons) so the entry uses the same artwork it browses.
var githubSVG = core.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#8a8a8e" d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>`))

// statusState feeds the bottom status bar.
type statusState struct {
	results *core.Signal[[]dataset.Icon]
	version string
	port    int
}

// statusBar shows the current result count, dataset version and MCP port on
// the left, and the GitHub repository entry on the right.
func statusBar(cx *core.Scope, s statusState) core.Node {
	return core.View{Class: "px-3 py-2 border-t border-neutral-800 flex-row items-center justify-between", Children: []core.Node{
		core.Text{ContentFunc: func() string {
			n := len(s.results.Get())
			if n > maxGridRows {
				return fmt.Sprintf("共 %d 个图标 · 浏览前 %d 个，输入关键词查看其余结果 · v%s · MCP :%d", n, maxGridRows, s.version, s.port)
			}
			return fmt.Sprintf("共 %d 个图标 · v%s · MCP :%d", n, s.version, s.port)
		}},
		ui.Press{
			OnPress: func() { cx.OpenURL(repoURL, nil) },
			Label:   "GitHub 仓库",
			Style:   core.Style{Padding: core.All(4), CornerRadius: 4},
			StyleFunc: func() core.Style {
				return core.Style{Hover: &core.Style{Background: core.RGB(0x262626)}}
			},
			Children: []core.Node{
				core.Icon{SVG: githubSVG, Style: core.Style{Width: core.Pt(16), Height: core.Pt(16)}},
			},
		},
	}}
}
