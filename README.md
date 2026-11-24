<p align="center">
  <img src="icon.png" alt="icons" width="128" height="128">
</p>

<h1 align="center">icons</h1>

<p align="center">English | <a href="README_CN.md">中文</a></p>

<p align="center">
A desktop browser and search tool for the entire <a href="https://github.com/react-icons/react-icons">react-icons</a> library — 50,934 vector icons across 31 sets, fully embedded in a single executable.<br>
Built-in MCP server for AI assistant integration.
</p>

## Screenshots

<p align="center">
  <img src="screenshot/icons-01.png" width="80%">
</p>
<p align="center">
  <img src="screenshot/icons-02.png" width="80%">
</p>

## Features

- **Icon Browsing** — Virtual-scrolling grid with vector rendering (sharp at any size/DPI); all icons are compiled into the binary, no downloads, no first-launch setup
- **Instant Search** — Debounced substring search over icon names and set keywords, with results capped and hinted past 10,000 rows
- **Set Sidebar** — All 31 icon sets (Font Awesome 6, Material Design, Tabler, Phosphor, Grommet, …) with live counts
- **Detail & Export** — Zoomed preview on a checkerboard, one-click SVG copy, PNG export and clipboard copy at any size and color
- **MCP Server** — Embedded MCP server (stdio + HTTP) for AI tools to search, fetch SVG and render PNG icons
- **Tray Resident** — Closing the window minimizes to the system tray; the MCP server keeps running. Single instance via a TCP port lock

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go |
| Desktop Framework | [nsgo](https://github.com/nsgo-dev/nsgo) (dark appearance, tray) |
| Dataset | [react-icons](https://github.com/react-icons/react-icons) v5.7.0, embedded (gzip JSON per set) |
| MCP | [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) |

## MCP Integration

The embedded MCP HTTP server starts automatically (default port 9393, changeable in settings). Alternatively, run `icons --mcp` for a standalone stdio server.

**Closing the window does not quit the app**: it minimizes to the system tray and keeps serving MCP. Click the tray icon to restore the window; the tray right-click menu offers *Show* / *Quit*.

### Available Tools

| Tool | Description |
|------|-------------|
| `list_sets` | List all icon sets (fa6, ri, tb, …) with icon counts and the react-icons version |
| `search_icons` | Search icons by keyword (name + set keywords), optionally filtered by set; returns up to 100 with the total count |
| `get_icon_svg` | Get an icon's raw SVG markup, optionally injecting a CSS color into the root `fill` |
| `export_icon_png` | Render an icon to PNG (base64), custom size (default 256) and color |

### Client Configuration

Add to your MCP client config (e.g. Claude Desktop, Cursor):

```json
{
  "mcpServers": {
    "icons": {
      "url": "http://127.0.0.1:9393/mcp"
    }
  }
}
```

## Build & Development

```bash
go tool nsgo dev            # dev run (hot reload of the app)
go tool nsgo build          # release build -> dist/releases/
go test ./internal/...
```

Plain `go build` also works; the icon resource comes from `rsrc_windows_amd64.syso`.

Windows CI builds an NSIS installer via `go tool nsgo build` on every push and publishes it on `v*` tags (see [.github/workflows/build-windows.yml](.github/workflows/build-windows.yml)).

## License

- Code: [MIT](LICENSE) © 2026 limengwei
- Icons: each react-icons set carries its own license (MIT / CC BY / SIL OFL / Apache 2.0 — see the set list in-app under *Settings → Data source*)
