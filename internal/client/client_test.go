package client

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchPushAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan Request, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var req Request
		_ = json.NewDecoder(conn).Decode(&req)
		received <- req
		enc := json.NewEncoder(conn)
		_ = enc.Encode(Response{OK: true, State: StateIdle})
		_ = enc.Encode(Response{OK: true, State: StateStreaming, Streams: []StreamInfo{{Device: "TV", State: StateStreaming}}})
		var b [1]byte
		conn.Read(b[:])
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var states []State
	err = (&Client{SocketPath: path}).Watch(ctx, func(r Response) {
		states = append(states, r.State)
		if len(states) == 2 {
			cancel()
		}
	})
	if err == nil {
		t.Fatal("expected cancellation to close watch")
	}
	if len(states) != 2 || states[0] != StateIdle || states[1] != StateStreaming {
		t.Fatal(states)
	}
	if req := <-received; req.Cmd != "watch" {
		t.Fatal(req)
	}
}
func TestDaemonErrorsAreReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		var r Request
		json.NewDecoder(c).Decode(&r)
		json.NewEncoder(c).Encode(Response{Error: "denied"})
	}()
	if _, err := (&Client{SocketPath: path}).Connect("tv", ""); err == nil {
		t.Fatal("daemon rejection ignored")
	}
}
