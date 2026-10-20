package main

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/404wolf/doubletake-bar/internal/client"
)

// waybarOutput matches the schema a waybar custom module with
// return-type=json expects.
type waybarOutput struct {
	Text    string `json:"text"`
	Tooltip string `json:"tooltip"`
	Class   string `json:"class"`
}

// runStatus is invoked on every waybar poll (see the `interval` on the
// custom/airplay module). It never blocks on anything but the daemon socket
// read, so a stuck daemon can't hang the bar.
func runStatus() {
	c := client.New()
	resp, err := c.Status()
	if err != nil {
		printWaybar(waybarOutput{
			Text:    appleIcon,
			Class:   "unavailable",
			Tooltip: "doubletake daemon is not running",
		})
		return
	}

	var streaming []string
	pinWaiting := false
	for _, s := range resp.Streams {
		switch s.State {
		case client.StateStreaming:
			streaming = append(streaming, s.Device)
		case client.StatePINRequired:
			pinWaiting = true
		}
	}

	switch {
	case len(streaming) > 0:
		printWaybar(waybarOutput{
			Text:    appleIcon,
			Class:   "streaming",
			Tooltip: "AirPlay: mirroring to " + strings.Join(streaming, ", "),
		})
	case pinWaiting:
		printWaybar(waybarOutput{
			Text:    appleIcon,
			Class:   "pairing",
			Tooltip: "AirPlay: waiting for PIN",
		})
	default:
		printWaybar(waybarOutput{
			Text:    appleIcon,
			Class:   "idle",
			Tooltip: "AirPlay: not mirroring",
		})
	}
}

// appleIcon is the Font Awesome "apple" glyph (nf-fa-apple). It renders
// consistently with this bar's other Nerd Font module icons, unlike an emoji
// glyph (e.g. 🖵), which sat noticeably off the bar's vertical center.
const appleIcon = ""

func printWaybar(o waybarOutput) {
	_ = json.NewEncoder(os.Stdout).Encode(o)
}
