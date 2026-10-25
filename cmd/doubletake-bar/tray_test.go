package main

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/404wolf/doubletake-bar/internal/client"
)

func TestTrayState(t *testing.T) {
	cases := []struct {
		r          client.Response
		state, tip string
	}{
		{client.Response{Error: "offline"}, "unavailable", "offline"},
		{client.Response{OK: true}, "idle", "AirPlay: not mirroring"},
		{client.Response{OK: true, Streams: []client.StreamInfo{{State: client.StateConnecting}}}, "connecting", "AirPlay: connecting"},
		{client.Response{OK: true, Streams: []client.StreamInfo{{State: client.StatePINRequired}}}, "connecting", "AirPlay: waiting for PIN or password"},
		{client.Response{OK: true, Streams: []client.StreamInfo{{Device: "TV", State: client.StateStreaming}, {State: client.StateConnecting}}}, "mirroring", "AirPlay: mirroring to TV"},
	}
	for _, tc := range cases {
		state, tip := trayState(tc.r)
		if state != tc.state || tip != tc.tip {
			t.Errorf("got %q %q, want %q %q", state, tip, tc.state, tc.tip)
		}
	}
}
func TestIcons(t *testing.T) {
	seen := map[string]bool{}
	for _, state := range []string{"idle", "connecting", "mirroring", "unavailable"} {
		b := trayIcon(state)
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != 32 {
			t.Fatal(img.Bounds())
		}
		if seen[string(b)] {
			t.Fatalf("icon for %s is indistinguishable", state)
		}
		seen[string(b)] = true
	}
}
