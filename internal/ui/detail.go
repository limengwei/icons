package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
	"reacticons/internal/exporter"
	"reacticons/internal/settings"
)

// detailState is the right-hand detail panel's inputs.
type detailState struct {
	icon    *core.Signal[dataset.Icon]
	ds      *dataset.Dataset
	svgs    *svgCache
	toaster *ui.Toaster
	onClose func()
}

// Detail-panel parameters: previewSize is the default PNG export size, and the
// palette starts on the grid's thumbnail color so both match.
const (
	previewSize  = 128
	previewColor = "#E5E5E5"
	previewBox   = 192
)

// paletteColors are the preset swatches offered for recoloring the icon.
var paletteColors = []string{
	"#E5E5E5", "#FFFFFF", "#000000", "#FF5722", "#E91E63", "#9C27B0", "#3F51B5",
	"#2196F3", "#00BCD4", "#009688", "#4CAF50", "#FFC107", "#FF9800", "#795548",
}

// bgOptions are the preview backgrounds (preview-only, never exported):
// "" is the transparency checkerboard, else a solid hex color.
var bgOptions = []struct{ Label, Value string }{
	{"透明", ""},
	{"黑", "#000000"},
	{"白", "#FFFFFF"},
}

// exportSizes are the pixel sizes offered for PNG export.
var exportSizes = []int{16, 32, 64, 128, 256, 512}

// detailPanel shows one icon large with its metadata, a recoloring palette
// and copy/export actions. Exports go through the system save dialog
// (cx.SaveFile), remembering the last folder saved in.
func detailPanel(cx *core.Scope, s detailState) core.Node {
	colorSig := core.NewSignal(cx, previewColor)
	hexSig := core.NewSignal(cx, previewColor) // color as typed in the hex field
	bgSig := core.NewSignal(cx, "")            // "": checkerboard, else solid hex
	sizeSig := core.NewSignal(cx, previewSize)
	// The panel edits its own settings copy: exports update LastSaveDir and
	// save it back so the next save dialog starts in the same folder.
	st, _ := settings.Load()

	// onHex applies a typed color when it parses as #RRGGBB (with or without
	// the leading #); anything else leaves the color untouched.
	onHex := func(v string) {
		hexSig.Set(v)
		if c := normalizeHex(v); c != "" {
			colorSig.Set(c)
		}
	}

	// svgNow returns the icon's raw SVG, toasting and reporting false when
	// the dataset cannot serve it.
	svgNow := func() (string, bool) {
		ic := s.icon.Get()
		svg, err := s.ds.GetSVG(ic.Set, ic.Name)
		if err != nil {
			s.toaster.Show(ui.ToastContent{Type: ui.ToastError, Title: "读取失败", Description: err.Error()})
			return "", false
		}
		return svg, true
	}

	// saveAs exports data through the system save dialog.
	saveAs := func(name, mime string, data []byte) {
		cx.SaveFile(core.SaveOptions{
			Name: name, MIME: mime, Data: data, Start: st.LastSaveDir,
		}, func(saved bool, path string, err error) {
			switch {
			case err != nil:
				s.toaster.Show(ui.ToastContent{Type: ui.ToastError, Title: "导出失败", Description: err.Error()})
			case saved:
				if dir := filepath.Dir(path); dir != "" {
					st.LastSaveDir = dir
					if err := st.Save(); err != nil {
						s.toaster.Show(ui.ToastContent{Type: ui.ToastError, Title: "记住目录失败", Description: err.Error()})
					}
				}
				s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已导出", Description: path, Duration: 8 * time.Second})
			}
		})
	}

	return core.View{Class: "w-80 flex-col border-l border-neutral-800 bg-neutral-900", Children: []core.Node{
		// Header.
		core.View{Class: "p-3 flex-row items-center justify-between border-b border-neutral-800", Children: []core.Node{
			core.Text{Content: "图标详情", Style: core.Style{FontSize: 14}},
			ui.Button{Text: "关闭", OnPress: s.onClose},
		}},

		// Body, scrolling vertically when the content outgrows the panel:
		// ui.ScrollView (a core.Scroll) is what actually scrolls — a plain
		// View with OverflowScroll only clips.
		ui.ScrollView{
			Style:        core.Style{Width: core.Pct(100)},
			ContentStyle: core.Style{Padding: core.All(16), Gap: 16},
			EdgeColor:    func() core.Color { return core.RGB(0x171717) },
			Children: []core.Node{
				// Large vector preview: the background is the transparency
				// checkerboard ("透明") or a solid black/white fill, preview-only —
				// exports never carry it. The icon is positioned absolute over
				// the background, fitted and centered, colored by
				// fill="currentColor".
				core.View{Class: "flex-row justify-center", Children: []core.Node{
					core.View{Style: core.Style{Width: core.Pt(224), Height: core.Pt(224), Position: core.Relative,
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
						StyleFunc: func() core.Style {
							if bgSig.Get() == "" {
								return core.Style{}
							}
							col, err := core.ParseColor(bgSig.Get())
							if err != nil {
								return core.Style{}
							}
							return core.Style{Background: col}
						}, Children: []core.Node{
							core.Show{When: func() bool { return bgSig.Get() == "" }, Then: func(cx *core.Scope) core.Node {
								return core.SVGView{
									SVG:   previewChecker,
									Style: core.Style{Width: core.Pct(100), Height: core.Pct(100)},
								}
							}},
							core.Icon{
								SVGFunc: func() *core.SVG {
									ic := s.icon.Get()
									return iconSVG(s.ds, s.svgs, ic.Set, ic.Name)
								},
								StyleFunc: func() core.Style {
									col, err := core.ParseColor(colorSig.Get())
									if err != nil {
										col = core.RGB(0xe5e5e5)
									}
									return core.Style{Position: core.Absolute, ZIndex: 1,
										Width: core.Pct(100), Height: core.Pct(100), Color: col}
								},
							},
						}},
				}},

				// Preview background choice (never exported).
				core.Text{Content: "预览背景", Style: core.Style{FontSize: 12, Color: core.RGB(0xa3a3a3)}},
				core.View{Class: "flex-row gap-2 flex-wrap", Children: bgButtons(bgSig)},

				// Color palette plus a hex value field.
				core.Text{Content: "颜色", Style: core.Style{FontSize: 12, Color: core.RGB(0xa3a3a3)}},
				core.View{Class: "flex-row gap-2 flex-wrap", Children: swatchButtons(colorSig, hexSig)},
				ui.Input{
					Value:       hexSig.Get,
					OnChange:    onHex,
					Placeholder: "自定义色值，如 #FF5722（可省略 #）",
				},

				core.Text{Content: "导出尺寸", Style: core.Style{FontSize: 12, Color: core.RGB(0xa3a3a3)}},
				core.View{Class: "flex-row gap-2 flex-wrap", Children: sizeButtons(sizeSig)},

				core.View{Class: "gap-2 flex-col", Children: []core.Node{
					ui.Button{Text: "复制 SVG", OnPress: func() {
						svg, ok := svgNow()
						if !ok {
							return
						}
						cx.SetClipboard(exporter.ApplyColor(svg, colorSig.Get()))
						s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已复制 SVG"})
					}},
					ui.Button{Text: "导出 SVG", OnPress: func() {
						svg, ok := svgNow()
						if !ok {
							return
						}
						saveAs(s.icon.Get().Name+".svg", "image/svg+xml", exporter.SVGBytes(svg, colorSig.Get()))
					}},
					ui.Button{TextFunc: func() string {
						return fmt.Sprintf("导出 PNG（%dpx）", sizeSig.Get())
					}, OnPress: func() {
						svg, ok := svgNow()
						if !ok {
							return
						}
						data, err := exporter.PNG(svg, sizeSig.Get(), colorSig.Get())
						if err != nil {
							s.toaster.Show(ui.ToastContent{Type: ui.ToastError, Title: "导出失败", Description: err.Error()})
							return
						}
						saveAs(fmt.Sprintf("%s_%dpx.png", s.icon.Get().Name, sizeSig.Get()), "image/png", data)
					}},
				}},

				infoRows(s),
			},
		},
	}}
}

// swatchButtons renders the palette as small pressable color chips with the
// selected one outlined; picking one also syncs the hex field.
func swatchButtons(color, hex *core.Signal[string]) []core.Node {
	out := make([]core.Node, 0, len(paletteColors))
	for _, c := range paletteColors {
		c := c
		face, err := core.ParseColor(c)
		if err != nil {
			continue
		}
		out = append(out, ui.Press{
			OnPress: func() {
				color.Set(c)
				hex.Set(c)
			},
			Label: c,
			Style: core.Style{Width: core.Pt(24), Height: core.Pt(24), CornerRadius: 6,
				Background: face, BorderWidth: 1, BorderColor: core.RGB(0x525252)},
			StyleFunc: func() core.Style {
				if color.Get() == c {
					return core.Style{BorderWidth: 2, BorderColor: core.RGB(0xffffff)}
				}
				return core.Style{}
			},
		})
	}
	return out
}

// bgButtons renders the preview-background choices (transparent / black /
// white) with the selected one highlighted.
func bgButtons(bg *core.Signal[string]) []core.Node {
	out := make([]core.Node, 0, len(bgOptions))
	for _, opt := range bgOptions {
		opt := opt
		out = append(out, ui.Press{
			OnPress: func() { bg.Set(opt.Value) },
			Label:   opt.Label,
			Style: core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 6,
				Background: core.RGB(0x262626), FontSize: 12},
			StyleFunc: func() core.Style {
				if bg.Get() == opt.Value {
					return core.Style{Background: core.RGB(0x3f3f46), Color: core.RGB(0xffffff)}
				}
				return core.Style{}
			},
			Children: []core.Node{core.Text{Content: opt.Label}},
		})
	}
	return out
}

// hexColorRe matches a 6-digit hex color with an optional leading #.
var hexColorRe = regexp.MustCompile(`^#?[0-9A-Fa-f]{6}$`)

// normalizeHex canonicalizes typed input to #RRGGBB, or returns "" when it is
// not a valid 6-digit hex color.
func normalizeHex(v string) string {
	v = strings.TrimSpace(v)
	if !hexColorRe.MatchString(v) {
		return ""
	}
	if !strings.HasPrefix(v, "#") {
		v = "#" + v
	}
	return strings.ToUpper(v)
}

func infoRows(s detailState) core.Node {
	row := func(label string, valueFunc func() string, style core.Style) core.Node {
		return core.View{Class: "gap-1 flex-col", Children: []core.Node{
			core.Text{Content: label, Style: core.Style{FontSize: 10, Color: core.RGB(0x737373)}},
			core.Text{ContentFunc: valueFunc, Style: style},
		}}
	}
	value := core.Style{FontSize: 12, TextOverflow: core.EllipsizeTail, Color: core.RGB(0xe5e5e5)}
	setInfo := func() (dataset.SetInfo, bool) { return s.ds.Set(s.icon.Get().Set) }
	return core.View{Class: "gap-3 flex-col", Children: []core.Node{
		core.Text{ContentFunc: func() string { return s.icon.Get().Name }, Style: core.Style{FontSize: 16}},
		row("集合", func() string {
			if info, ok := setInfo(); ok {
				return fmt.Sprintf("%s · %s", info.ID, info.Name)
			}
			return s.icon.Get().Set
		}, value),
		row("许可证", func() string {
			if info, ok := setInfo(); ok {
				return info.License
			}
			return ""
		}, value),
		row("项目地址", func() string {
			if info, ok := setInfo(); ok {
				return info.ProjectURL
			}
			return ""
		}, value),
		row("import 语句", func() string {
			ic := s.icon.Get()
			return exporter.Import(ic.Name, ic.Set)
		}, core.Style{FontSize: 11, FontFamily: core.FontMonospace, TextOverflow: core.EllipsizeTail, Color: core.RGB(0xe5e5e5)}),
	}}
}

// sizeButtons renders the export-size choices with the selected one
// highlighted.
func sizeButtons(size *core.Signal[int]) []core.Node {
	out := make([]core.Node, 0, len(exportSizes))
	for _, sz := range exportSizes {
		sz := sz
		out = append(out, ui.Press{
			OnPress: func() { size.Set(sz) },
			Style: core.Style{Padding: core.Symmetric(4, 10), CornerRadius: 6,
				Background: core.RGB(0x262626), FontSize: 12},
			StyleFunc: func() core.Style {
				if size.Get() == sz {
					return core.Style{Background: core.RGB(0x3f3f46), Color: core.RGB(0xffffff)}
				}
				return core.Style{}
			},
			Children: []core.Node{core.Text{Content: fmt.Sprintf("%d", sz)}},
		})
	}
	return out
}
