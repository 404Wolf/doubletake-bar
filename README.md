# doubletake-bar

A waybar status module and GTK dropdown for [doubletake](https://github.com/omarroth/doubletake),
an AirPlay mirroring *sender* for Linux. Lets you see and switch AirPlay
targets from a bar icon the way macOS's AirPlay menu works, instead of the
command line.

Talks to a running `doubletake -daemonize` over its Unix control socket
(`$XDG_RUNTIME_DIR/doubletake.sock`).

## Commands

- `doubletake-bar status` — prints one line of [waybar `custom/*` module
  JSON](https://github.com/Alexays/Waybar/wiki/Module:-Custom) (`text`,
  `tooltip`, `class`) reflecting whether doubletake is idle, streaming, or
  waiting for a pairing PIN. Meant to be polled on an interval.
- `doubletake-bar popup` — opens (or, if already open, closes) a small
  dropdown anchored top-right via `gtk-layer-shell`, listing devices
  discovered by the daemon. Click a device to connect; click the active one
  to disconnect. A device that needs pairing shows an inline PIN entry.
  Running it again while the popup is already up just toggles it, via its
  own control socket (`$XDG_RUNTIME_DIR/doubletake-bar.sock`) — so one waybar
  `on-click` handler works for both opening and closing it.

## Waybar wiring

```jsonc
"custom/airplay": {
  "exec": "doubletake-bar status",
  "return-type": "json",
  "interval": 5,
  "on-click": "doubletake-bar popup"
}
```

## Build

```sh
nix build
# or, for local iteration:
nix develop
go build -tags gtk_3_24 ./cmd/doubletake-bar
```

The `gtk_3_24` build tag works around a gotk3 upstream bug where its
version-detection build constraints miscompile against recent GTK3.
