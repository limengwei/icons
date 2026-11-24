package ui

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/ui"

	"reacticons/internal/dataset"
	"reacticons/internal/exporter"
	"reacticons/internal/render"
	"reacticons/internal/settings"
)

// detailState is the right-hand detail panel's inputs.
type detailState struct {
	icon    *core.Signal[dataset.Icon]
	ds      *dataset.Dataset
	cache   *render.Cache
	toaster *ui.Toaster
	onClose func()
}

// Detail-panel parameters: the preview rasterizes at 128px in the selected
// color; the palette starts on the grid's thumbnail color so both match.
const (
	previewSize  = 128
	previewColor = gridColor
)

// paletteColors are the preset swatches offered for recoloring the icon.
var paletteColors = []string{
	"#E5E5E5", "#FFFFFF", "#000000", "#FF5722", "#E91E63", "#9C27B0", "#3F51B5",
	"#2196F3", "#00BCD4", "#009688", "#4CAF50", "#FFC107", "#FF9800", "#795548",
}

// exportSizes are the pixel sizes offered for PNG export.
var exportSizes = []int{16, 32, 48, 64, 128, 256, 512}

// detailPanel shows one icon large with its metadata, a recoloring palette
// and copy/export actions. Exports go through the system save dialog
// (cx.SaveFile), remembering the last folder saved in.
func detailPanel(cx *core.Scope, s detailState) core.Node {
	colorSig := core.NewSignal(cx, previewColor)
	sizeSig := core.NewSignal(cx, previewSize)
	refreshSig := core.NewSignal(cx, 0) // bumped when a preview rasterization lands
	// The panel edits its own settings copy: exports update LastSaveDir and
	// save it back so the next save dialog starts in the same folder.
	st, _ := settings.Load()

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

		// Body.
		core.View{Class: "flex-1 min-h-0 overflow-y-auto p-4 gap-4 flex-col", Children: []core.Node{
			// Large preview, rasterized in the background at previewSize in
			// the selected color.
			core.View{Class: "flex-row justify-center", Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(224), Height: core.Pt(224), CornerRadius: 12,
					Background: core.RGB(0x1c1c1e), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, Children: []core.Node{
					core.Image{
						SourceFunc: func() core.ImageSource {
							refreshSig.Get() // depend on rasterization arrivals
							ic := s.icon.Get()
							key := fmt.Sprintf("%s|%s|%d|%s", ic.Set, ic.Name, previewSize, colorSig.Get())
							if img, ok := s.cache.Get(key); ok {
								return core.ImageSource{Pixels: img, Size: core.Size{Width: core.Points(img.Bounds().Dx()), Height: core.Points(img.Bounds().Dy())}}
							}
							startPreviewRasterize(cx, s, refreshSig, key, ic, colorSig.Get())
							return core.ImageSource{}
						},
						ResizeMode: core.Contain,
						Style:      core.Style{Width: core.Pct(100), Height: core.Pct(100)},
					},
				}},
			}},

			// Color palette.
			core.Text{Content: "颜色", Style: core.Style{FontSize: 12, Color: core.RGB(0xa3a3a3)}},
			core.View{Class: "flex-row gap-2 flex-wrap", Children: swatchButtons(colorSig)},

			infoRows(s),

			core.Text{Content: "导出尺寸", Style: core.Style{FontSize: 12, Color: core.RGB(0xa3a3a3)}},
			core.View{Class: "flex-row gap-2 flex-wrap", Children: sizeButtons(sizeSig)},

			core.View{Class: "gap-2 flex-col", Children: []core.Node{
				ui.Button{Text: "复制 JSX", OnPress: func() {
					jsx := exporter.JSX(s.icon.Get().Name)
					cx.SetClipboard(jsx)
					s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已复制 JSX", Description: jsx})
				}},
				ui.Button{Text: "复制 import", OnPress: func() {
					ic := s.icon.Get()
					imp := exporter.Import(ic.Name, ic.Set)
					cx.SetClipboard(imp)
					s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已复制 import", Description: imp})
				}},
				ui.Button{Text: "复制 SVG", OnPress: func() {
					svg, ok := svgNow()
					if !ok {
						return
					}
					cx.SetClipboard(exporter.ApplyColor(svg, colorSig.Get()))
					s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已复制 SVG"})
				}},
				ui.Button{Text: "复制组件代码", OnPress: func() {
					ic := s.icon.Get()
					svg, ok := svgNow()
					if !ok {
						return
					}
					cx.SetClipboard(exporter.Component(ic.Name, svg))
					s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已复制组件代码"})
				}},
				ui.Button{TextFunc: func() string {
					return fmt.Sprintf("复制 PNG（%dpx）", sizeSig.Get())
				}, OnPress: func() {
					svg, ok := svgNow()
					if !ok {
						return
					}
					data, err := exporter.PNG(svg, sizeSig.Get(), colorSig.Get())
					if err != nil {
						s.toaster.Show(ui.ToastContent{Type: ui.ToastError, Title: "复制失败", Description: err.Error()})
						return
					}
					cx.SetClipboardImage(data)
					s.toaster.Show(ui.ToastContent{Type: ui.ToastSuccess, Title: "已复制 PNG"})
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
		}},
	}}
}

// startPreviewRasterize renders the detail preview in the background and
// bumps refresh when the image lands in the cache, deduplicating through the
// same in-flight map as the grid.
func startPreviewRasterize(cx *core.Scope, s detailState, refresh *core.Signal[int], key string, ic dataset.Icon, color string) {
	inFlightMu.Lock()
	if inFlight[key] {
		inFlightMu.Unlock()
		return
	}
	inFlight[key] = true
	inFlightMu.Unlock()

	go func() {
		defer func() {
			inFlightMu.Lock()
			delete(inFlight, key)
			inFlightMu.Unlock()
		}()
		svg, err := s.ds.GetSVG(ic.Set, ic.Name)
		if err != nil {
			return
		}
		img, err := render.Rasterize(svg, previewSize, color)
		if err != nil {
			return
		}
		s.cache.Put(key, img)
		cx.Post(func() { refresh.Set(refresh.Get() + 1) })
	}()
}

// swatchButtons renders the palette as small pressable color chips with the
// selected one outlined.
func swatchButtons(color *core.Signal[string]) []core.Node {
	out := make([]core.Node, 0, len(paletteColors))
	for _, c := range paletteColors {
		c := c
		face, err := core.ParseColor(c)
		if err != nil {
			continue
		}
		out = append(out, ui.Press{
			OnPress: func() { color.Set(c) },
			Label:   c,
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
