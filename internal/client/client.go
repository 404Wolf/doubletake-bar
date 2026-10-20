// Package client talks to a running doubletake daemon (doubletake -daemonize)
// over its Unix control socket. The wire format mirrors doubletake's own
// internal/daemon package, which is unexported and so can't be imported
// directly: https://github.com/omarroth/doubletake/blob/master/internal/daemon/daemon.go
package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

type State string

const (
	StateIdle        State = "idle"
	StateDiscovering State = "discovering"
	StateConnecting  State = "connecting"
	StateStreaming   State = "streaming"
	StatePINRequired State = "pin_required"
)

type CredentialKind string

const (
	CredentialKindPIN      CredentialKind = "pin"
	CredentialKindPassword CredentialKind = "password"
)

type Request struct {
	Cmd    string `json:"cmd"`
	Target string `json:"target,omitempty"`
	Port   int    `json:"port,omitempty"`
	Pin    string `json:"pin,omitempty"`
}

type DeviceInfo struct {
	Name     string `json:"name"`
	Model    string `json:"model"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	DeviceID string `json:"device_id"`
}

type StreamInfo struct {
	Device         string         `json:"device"`
	DeviceIP       string         `json:"device_ip"`
	State          State          `json:"state"`
	HasAudio       bool           `json:"has_audio"`
	AudioMuted     bool           `json:"audio_muted"`
	CredentialKind CredentialKind `json:"credential_kind,omitempty"`
}

type Response struct {
	OK              bool           `json:"ok"`
	State           State          `json:"state"`
	Device          string         `json:"device,omitempty"`
	DeviceIP        string         `json:"device_ip,omitempty"`
	HasAudio        bool           `json:"has_audio"`
	AudioMuted      bool           `json:"audio_muted"`
	NeedsPIN        bool           `json:"needs_pin,omitempty"`
	NeedsCredential bool           `json:"needs_credential,omitempty"`
	CredentialKind  CredentialKind `json:"credential_kind,omitempty"`
	Error           string         `json:"error,omitempty"`
	Devices         []DeviceInfo   `json:"devices,omitempty"`
	Streams         []StreamInfo   `json:"streams,omitempty"`
}

// Client is a connection-per-request client for the doubletake daemon socket.
type Client struct {
	SocketPath string
}

// DefaultSocketPath matches doubletake's daemon.DefaultSocketPath().
func DefaultSocketPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = "/tmp"
	}
	return filepath.Join(dir, "doubletake.sock")
}

func New() *Client {
	return &Client{SocketPath: DefaultSocketPath()}
}

func (c *Client) send(req Request) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.SocketPath, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to doubletake daemon: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return &resp, nil
}

func (c *Client) Status() (*Response, error) { return c.send(Request{Cmd: "status"}) }

func (c *Client) Discover() (*Response, error) { return c.send(Request{Cmd: "discover"}) }

func (c *Client) Devices() (*Response, error) { return c.send(Request{Cmd: "devices"}) }

// Connect starts mirroring to target (or the first free discovered device if
// target is ""). When pin is non-empty it submits pairing credentials for a
// device that's waiting on them (see Response.State == StatePINRequired).
func (c *Client) Connect(target, pin string) (*Response, error) {
	return c.send(Request{Cmd: "connect", Target: target, Pin: pin})
}

// Disconnect stops mirroring to target, or every active stream if target is "".
func (c *Client) Disconnect(target string) (*Response, error) {
	return c.send(Request{Cmd: "disconnect", Target: target})
}

func (c *Client) Mute(target string) (*Response, error) {
	return c.send(Request{Cmd: "mute", Target: target})
}

func (c *Client) Unmute(target string) (*Response, error) {
	return c.send(Request{Cmd: "unmute", Target: target})
}

// Running reports whether the daemon socket is reachable at all.
func (c *Client) Running() bool {
	conn, err := net.DialTimeout("unix", c.SocketPath, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
