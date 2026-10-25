package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"fyne.io/systray"
	"github.com/404wolf/doubletake-bar/internal/client"
	"github.com/404wolf/doubletake-bar/internal/logging"
	"github.com/godbus/dbus/v5"
	"go.uber.org/zap"
)

// A single controller owns menu replacement; old click handlers are cancelled
// on each snapshot so disappearing receivers cannot leak goroutines.
func runTray() error {
	bus, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer bus.Close()
	reply, err := bus.RequestName("com.github.doubletake.Bar", dbus.NameFlagDoNotQueue)
	if err != nil {
		return err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("AirPlay tray is already running")
	}
	log = logging.New("tray")
	defer log.Sync()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	c := client.New()
	snapshots := make(chan client.Response, 1)
	publish := func(r client.Response) {
		select {
		case <-snapshots:
		default:
		}
		snapshots <- r
	}
	systray.Run(func() {
		systray.SetTitle("AirPlay")
		go func() {
			for ctx.Err() == nil {
				err := c.Watch(ctx, publish)
				if ctx.Err() != nil {
					return
				}
				publish(client.Response{Error: "doubletake unavailable: " + err.Error()})
				log.Warn("daemon subscription", zap.Error(err))
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}()
		go func() {
			var stop context.CancelFunc = func() {}
			defer func() { stop() }()
			for {
				select {
				case <-ctx.Done():
					systray.Quit()
					return
				case r := <-snapshots:
					stop()
					stop = renderMenu(ctx, c, r)
				}
			}
		}()
	}, cancel)
	return nil
}

func trayState(r client.Response) (string, string) {
	if !r.OK {
		return "unavailable", r.Error
	}
	var names []string
	connecting, pairing := false, false
	for _, s := range r.Streams {
		switch s.State {
		case client.StateStreaming:
			names = append(names, s.Device)
		case client.StatePINRequired:
			pairing = true
		default:
			connecting = true
		}
	}
	if len(names) > 0 {
		return "mirroring", "AirPlay: mirroring to " + strings.Join(names, ", ")
	}
	if pairing {
		return "connecting", "AirPlay: waiting for PIN or password"
	}
	if connecting {
		return "connecting", "AirPlay: connecting"
	}
	return "idle", "AirPlay: not mirroring"
}

func renderMenu(parent context.Context, c *client.Client, r client.Response) context.CancelFunc {
	ctx, cancel := context.WithCancel(parent)
	state, tip := trayState(r)
	systray.SetIcon(trayIcon(state))
	systray.SetTooltip(tip)
	systray.ResetMenu()
	systray.AddMenuItem("AirPlay", "").Disable()
	action := func(item *systray.MenuItem, f func() error) {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-item.ClickedCh:
					if err := f(); err != nil {
						log.Warn("menu action", zap.Error(err))
						showError(err)
					}
				}
			}
		}()
	}
	streams := map[string]client.StreamInfo{}
	for _, s := range r.Streams {
		streams[s.DeviceIP] = s
	}
	// Include active streams even if discovery no longer lists the receiver.
	devices := append([]client.DeviceInfo(nil), r.Devices...)
	for _, s := range r.Streams {
		found := false
		for _, d := range devices {
			if d.IP == s.DeviceIP {
				found = true
				break
			}
		}
		if !found {
			devices = append(devices, client.DeviceInfo{Name: s.Device, IP: s.DeviceIP})
		}
	}
	for _, d := range devices {
		d := d
		s, active := streams[d.IP]
		label := strings.ReplaceAll(d.Name, "_", "__")
		if s.State == client.StateConnecting {
			label += " (connecting…)"
		}
		item := systray.AddMenuItemCheckbox(label, d.IP, s.State == client.StateStreaming)
		action(item, func() error {
			if active {
				if s.State == client.StatePINRequired {
					return submitCredential(c, d.IP, s.CredentialKind)
				}
				_, err := c.Disconnect(d.IP)
				return err
			}
			_, err := c.Connect(d.IP, "")
			return err
		})
	}
	if len(devices) == 0 {
		label := "No AirPlay receivers found"
		if !r.OK {
			label = "doubletake daemon unavailable"
		}
		systray.AddMenuItem(label, tip).Disable()
	}
	systray.AddSeparator()
	if len(r.Streams) > 0 {
		action(systray.AddMenuItem("Stop mirroring", "Disconnect all receivers"), func() error { _, err := c.Disconnect(""); return err })
		for _, s := range r.Streams {
			s := s
			sub := systray.AddMenuItem(strings.ReplaceAll(s.Device, "_", "__"), s.DeviceIP)
			action(sub.AddSubMenuItem("Disconnect", ""), func() error { _, err := c.Disconnect(s.DeviceIP); return err })
			if s.State == client.StatePINRequired {
				title := "Enter PIN…"
				if s.CredentialKind == client.CredentialKindPassword {
					title = "Enter password…"
				}
				action(sub.AddSubMenuItem(title, ""), func() error { return submitCredential(c, s.DeviceIP, s.CredentialKind) })
			}
			if s.HasAudio {
				mute := sub.AddSubMenuItemCheckbox("Mute audio", "", s.AudioMuted)
				action(mute, func() error {
					if s.AudioMuted {
						_, err := c.Unmute(s.DeviceIP)
						return err
					}
					_, err := c.Mute(s.DeviceIP)
					return err
				})
			}
		}
	}
	if r.OK && r.Error != "" {
		systray.AddMenuItem(r.Error, "").Disable()
	}
	action(systray.AddMenuItem("Rescan", "Refresh discovered receivers"), func() error { _, err := c.Discover(); return err })
	return cancel
}

// dbusmenu has no text-entry widget; credentials use a standard GTK dialog.
// Credentials return over a pipe from zenity and travel over the private daemon socket.
func submitCredential(c *client.Client, target string, kind client.CredentialKind) error {
	label := "Enter the PIN shown on the receiver"
	if kind == client.CredentialKindPassword {
		label = "Enter the receiver password"
	}
	cmd := exec.Command("zenity", "--entry", "--hide-text", "--title=AirPlay", "--text="+label)
	out, err := cmd.Output()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 {
		return nil
	}
	if err != nil {
		return fmt.Errorf("credential dialog: %w", err)
	}
	credential := strings.TrimRight(string(out), "\r\n")
	if credential == "" {
		return nil
	}
	_, err = c.Connect(target, credential)
	return err
}
func showError(err error) {
	_ = exec.Command("zenity", "--error", "--title=AirPlay", "--text="+err.Error()).Run()
}

// Small, scale-independent source artwork rasterized at 32px for SNI IconPixmap.
func trayIcon(state string) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{210, 210, 210, 255}
	if state == "mirroring" {
		ink = color.NRGBA{100, 200, 255, 255}
	}
	if state == "connecting" {
		ink = color.NRGBA{255, 190, 80, 255}
	}
	if state == "unavailable" {
		ink = color.NRGBA{120, 120, 120, 255}
	}
	for y := 4; y < 24; y++ {
		for x := 2; x < 30; x++ {
			if y < 7 || y > 20 || x < 5 || x > 26 {
				img.SetNRGBA(x, y, ink)
			}
		}
	}
	for y := 17; y < 30; y++ {
		for x := 16 - (y - 17); x <= 16+(y-17); x++ {
			if x >= 4 && x < 29 {
				img.SetNRGBA(x, y, ink)
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}
