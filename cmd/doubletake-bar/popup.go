package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/404wolf/doubletake-bar/internal/client"
	"github.com/dlasky/gotk3-layershell/layershell"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"go.uber.org/zap"
)

// controlSocketPath is doubletake-bar's own control socket (distinct from
// doubletake's own daemon socket): a running popup instance listens on it so
// a second `doubletake-bar popup` invocation (another waybar click) can just
// tell it to toggle instead of starting a second GTK process.
func controlSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "doubletake-bar.sock")
}

const css = `
window.doubletake-popup {
	background-color: #0d0d0d;
	border: 1px solid #333333;
	border-radius: 10px;
}
window.doubletake-popup box.content {
	padding: 8px;
}
window.doubletake-popup label.title {
	color: #e4e4e4;
	font-weight: bold;
	padding: 2px 6px 6px 6px;
}
window.doubletake-popup row {
	padding: 6px 8px;
	border-radius: 6px;
	color: #e4e4e4;
}
window.doubletake-popup row:hover {
	background-color: #1f1f1f;
}
window.doubletake-popup row.active label {
	color: #7fd1ff;
}
window.doubletake-popup label.empty {
	color: #7f7f7f;
	padding: 10px;
}
window.doubletake-popup entry {
	margin: 4px 2px 0 2px;
}
`

// runPopup is the `doubletake-bar popup` entrypoint: toggle an existing
// window if one's already running, otherwise become that instance.
func runPopup() {
	sock := controlSocketPath()
	if conn, err := net.DialTimeout("unix", sock, 500*time.Millisecond); err == nil {
		fmt.Fprint(conn, "toggle\n")
		conn.Close()
		return
	}
	_ = os.Remove(sock) // stale socket from a crashed instance

	listener, err := net.Listen("unix", sock)
	if err != nil {
		log.Fatal("listen on control socket", zap.String("path", sock), zap.Error(err))
	}
	defer listener.Close()

	gtk.Init(nil)

	provider, err := gtk.CssProviderNew()
	if err != nil {
		log.Fatal("create css provider", zap.Error(err))
	}
	if err := provider.LoadFromData(css); err != nil {
		log.Fatal("load css", zap.Error(err))
	}
	if screen, err := gdk.ScreenGetDefault(); err == nil && screen != nil {
		gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	p := newPopup()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
			glib.IdleAdd(p.toggle)
		}
	}()

	p.show()
	log.Info("popup ready")
	gtk.Main()
}

type popup struct {
	win        *gtk.Window
	listHolder *gtk.Box
	list       *gtk.ListBox
	client     *client.Client
	visible    bool
	shownAt    time.Time
	hasTimeout bool

	rows []client.DeviceInfo
}

func newPopup() *popup {
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		log.Fatal("create window", zap.Error(err))
	}
	sc, _ := win.GetStyleContext()
	sc.AddClass("doubletake-popup")
	win.SetDefaultSize(260, -1)
	win.SetDecorated(false)

	layershell.InitForWindow(win)
	layershell.SetNamespace(win, "doubletake-bar")
	layershell.SetLayer(win, layershell.LAYER_SHELL_LAYER_OVERLAY)
	layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_TOP, true)
	layershell.SetAnchor(win, layershell.LAYER_SHELL_EDGE_LEFT, true)
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_TOP, 34)
	// Approximates where the bar icon sits among modules-left; waybar is a
	// separate process so this can't be read live and may need hand-tuning
	// if modules-left changes width (e.g. workspace count, weather string).
	layershell.SetMargin(win, layershell.LAYER_SHELL_EDGE_LEFT, 110)
	layershell.SetKeyboardMode(win, layershell.LAYER_SHELL_KEYBOARD_MODE_ON_DEMAND)

	content, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
	contentSc, _ := content.GetStyleContext()
	contentSc.AddClass("content")

	title, _ := gtk.LabelNew("AirPlay")
	title.SetHAlign(gtk.ALIGN_START)
	titleSc, _ := title.GetStyleContext()
	titleSc.AddClass("title")
	content.PackStart(title, false, false, 0)

	listHolder, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	content.PackStart(listHolder, true, true, 0)

	win.Add(content)

	p := &popup{win: win, listHolder: listHolder, client: client.New()}

	win.Connect("key-press-event", func(_ *gtk.Window, ev *gdk.Event) bool {
		keyEvent := gdk.EventKeyNewFromEvent(ev)
		if keyEvent.KeyVal() == gdk.KEY_Escape {
			p.hide()
			return true
		}
		return false
	})
	// Hyprland briefly grants then revokes keyboard focus to a freshly
	// mapped on-demand-keyboard layer surface; honoring focus-out instantly
	// raced that and tore the surface down right after it appeared. A short
	// grace period after show() lets that settle while still dismissing on
	// a genuine click outside.
	win.Connect("focus-out-event", func() bool {
		if p.visible && time.Since(p.shownAt) > 300*time.Millisecond {
			p.hide()
		}
		return false
	})
	p.replaceList()
	return p
}

// replaceList swaps in a fresh, empty GtkListBox. Rebuilding from scratch on
// every refresh is simpler and less error-prone than diffing rows in gotk3.
func (p *popup) replaceList() {
	if p.list != nil {
		p.listHolder.Remove(p.list)
	}
	list, _ := gtk.ListBoxNew()
	list.SetSelectionMode(gtk.SELECTION_NONE)
	list.SetActivateOnSingleClick(true)
	list.Connect("row-activated", func(_ *gtk.ListBox, row *gtk.ListBoxRow) {
		idx := row.GetIndex()
		if idx < 0 || idx >= len(p.rows) {
			return
		}
		p.onRowActivated(p.rows[idx])
	})
	p.listHolder.Add(list)
	p.list = list
	p.rows = nil
}

func (p *popup) toggle() bool {
	if p.visible {
		p.hide()
	} else {
		p.show()
	}
	return false
}

func (p *popup) show() {
	p.refresh()
	p.win.ShowAll()
	p.win.Present()
	p.visible = true
	p.shownAt = time.Now()
	if !p.hasTimeout {
		p.hasTimeout = true
		glib.TimeoutAdd(2000, func() bool {
			if p.visible {
				p.refresh()
			}
			return true
		})
	}
}

func (p *popup) hide() {
	p.win.Hide()
	p.visible = false
}

type deviceState struct {
	active   bool
	needsPIN bool
}

// refresh rebuilds the device list from the daemon's current state.
func (p *popup) refresh() {
	p.replaceList()

	devResp, err := p.client.Devices()
	if err != nil {
		p.addEmptyRow("doubletake daemon not running")
		p.list.ShowAll()
		return
	}

	states := map[string]deviceState{}
	if statusResp, err := p.client.Status(); err == nil {
		for _, s := range statusResp.Streams {
			switch s.State {
			case client.StateStreaming:
				states[s.DeviceIP] = deviceState{active: true}
			case client.StatePINRequired:
				states[s.DeviceIP] = deviceState{needsPIN: true}
			}
		}
	}

	if len(devResp.Devices) == 0 {
		p.addEmptyRow("No AirPlay devices found")
	}
	for _, dev := range devResp.Devices {
		st := states[dev.IP]
		p.addDeviceRow(dev, st.active, st.needsPIN)
	}

	p.list.ShowAll()
}

func (p *popup) addEmptyRow(text string) {
	label, _ := gtk.LabelNew(text)
	label.SetHAlign(gtk.ALIGN_START)
	sc, _ := label.GetStyleContext()
	sc.AddClass("empty")
	p.list.Add(label)
}

func (p *popup) addDeviceRow(dev client.DeviceInfo, active, needsPIN bool) {
	row, _ := gtk.ListBoxRowNew()
	rowBox, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)

	line, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)

	mark := "  "
	if active {
		mark = "✓ "
	}
	markLabel, _ := gtk.LabelNew(mark)
	line.PackStart(markLabel, false, false, 0)

	nameLabel, _ := gtk.LabelNew(dev.Name)
	nameLabel.SetHAlign(gtk.ALIGN_START)
	line.PackStart(nameLabel, true, true, 0)

	rowBox.PackStart(line, false, false, 0)

	if needsPIN {
		entry, _ := gtk.EntryNew()
		entry.SetPlaceholderText("Enter PIN shown on screen")
		target := dev.IP
		entry.Connect("activate", func() {
			pin, _ := entry.GetText()
			if _, err := p.client.Connect(target, pin); err != nil {
				log.Warn("submit pin", zap.String("target", target), zap.Error(err))
			}
			p.refresh()
		})
		rowBox.PackStart(entry, false, false, 0)
	}

	row.Add(rowBox)
	if active {
		rsc, _ := row.GetStyleContext()
		rsc.AddClass("active")
	}

	p.rows = append(p.rows, dev)
	p.list.Add(row)
}

func (p *popup) onRowActivated(dev client.DeviceInfo) {
	statusResp, err := p.client.Status()
	if err != nil {
		return
	}
	for _, s := range statusResp.Streams {
		if s.DeviceIP != dev.IP {
			continue
		}
		switch s.State {
		case client.StateStreaming:
			if _, err := p.client.Disconnect(dev.IP); err != nil {
				log.Warn("disconnect", zap.String("target", dev.IP), zap.Error(err))
			}
		case client.StatePINRequired:
			return // handled by the row's own PIN entry
		default:
			return // already connecting
		}
		p.refresh()
		return
	}
	if _, err := p.client.Connect(dev.IP, ""); err != nil {
		log.Warn("connect", zap.String("target", dev.IP), zap.Error(err))
	}
	p.refresh()
}
