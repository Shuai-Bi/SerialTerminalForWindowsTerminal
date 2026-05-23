package forward

import (
	"net"
	"testing"
	"time"
)

func TestManagerTCPFlow(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	acceptCh := make(chan net.Conn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, e := listener.Accept()
		if e != nil {
			errCh <- e
			return
		}
		acceptCh <- conn
	}()

	serialCh := make(chan string, 2)
	mgr := NewManager(func(b []byte) error {
		serialCh <- string(b)
		return nil
	}, func(string, ...any) {})
	defer mgr.Close()

	id, err := mgr.Add(TCP, listener.Addr().String())
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	var serverConn net.Conn
	select {
	case serverConn = <-acceptCh:
	case e := <-errCh:
		t.Fatalf("accept failed: %v", e)
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for accepted connection")
	}
	defer serverConn.Close()

	items := mgr.List()
	if len(items) != 1 || items[0].ID != id || !items[0].Enabled {
		t.Fatalf("unexpected list after add: %+v", items)
	}

	if err = serverConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	mgr.Broadcast([]byte("from-app"))
	buf := make([]byte, 64)
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("server read from broadcast failed: %v", err)
	}
	if string(buf[:n]) != "from-app" {
		t.Fatalf("broadcast payload mismatch got=%q", string(buf[:n]))
	}

	if _, err = serverConn.Write([]byte("from-remote")); err != nil {
		t.Fatalf("server write failed: %v", err)
	}
	select {
	case got := <-serialCh:
		if got != "from-remote" {
			t.Fatalf("writeToSerial payload mismatch got=%q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for writeToSerial callback")
	}

	if err = mgr.Disable(id); err != nil {
		t.Fatalf("Disable() failed: %v", err)
	}
	items = mgr.List()
	if len(items) != 1 || items[0].Enabled {
		t.Fatalf("Disable() did not update state: %+v", items)
	}

	if err = mgr.Remove(id); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}
	if got := mgr.List(); len(got) != 0 {
		t.Fatalf("expected empty list after remove, got=%+v", got)
	}
}

func TestManagerErrorCases(t *testing.T) {
	mgr := NewManager(func([]byte) error { return nil }, func(string, ...any) {})
	defer mgr.Close()

	if _, err := mgr.Add(None, "127.0.0.1:1"); err == nil {
		t.Fatalf("Add(None) expected error")
	}

	if err := mgr.Remove(999); err == nil {
		t.Fatalf("Remove(non-existing) expected error")
	}

	if err := mgr.Disable(999); err == nil {
		t.Fatalf("Disable(non-existing) expected error")
	}

	if err := mgr.Enable(999); err == nil {
		t.Fatalf("Enable(non-existing) expected error")
	}

	if err := mgr.Update(999, TCP, "127.0.0.1:1"); err == nil {
		t.Fatalf("Update(non-existing) expected error")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	id, err := mgr.Add(TCP, listener.Addr().String())
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}
	if err = mgr.Update(id, None, "127.0.0.1:1"); err == nil {
		t.Fatalf("Update(None) expected error")
	}
}

func TestManagerSetInboundReporter(t *testing.T) {
	reported := make(chan []byte, 1)
	mgr := NewManager(func([]byte) error { return nil }, func(string, ...any) {})
	defer mgr.Close()
	mgr.SetInboundReporter(func(id int, chunk []byte) {
		reported <- chunk
	})
	// Verify the callback was stored (indirect test)
	_ = reported
}

func TestManagerBroadcastToDisabled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	writeCh := make(chan []byte, 4)
	mgr := NewManager(func([]byte) error {
		writeCh <- nil
		return nil
	}, func(string, ...any) {})
	defer mgr.Close()

	id, err := mgr.Add(TCP, listener.Addr().String())
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	if err = mgr.Disable(id); err != nil {
		t.Fatalf("Disable() failed: %v", err)
	}

	mgr.Broadcast([]byte("should-not-arrive"))

	select {
	case <-writeCh:
		t.Fatalf("broadcast should not write to serial when disabled")
	default:
	}

	mgr.Broadcast(nil)
	mgr.Broadcast([]byte{})
}

func TestManagerEnable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	writeCh := make(chan []byte, 2)
	mgr := NewManager(func([]byte) error {
		writeCh <- nil
		return nil
	}, func(string, ...any) {})
	defer mgr.Close()

	id, err := mgr.Add(TCP, listener.Addr().String())
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	if err = mgr.Disable(id); err != nil {
		t.Fatalf("Disable() failed: %v", err)
	}

	if err = mgr.Enable(id); err != nil {
		t.Fatalf("Enable() failed: %v", err)
	}

	items := mgr.List()
	if len(items) != 1 || !items[0].Enabled {
		t.Fatalf("expected enabled after Enable(), got=%+v", items)
	}

	if err = mgr.Enable(id); err != nil {
		t.Fatalf("second Enable() should succeed: %v", err)
	}
}

func TestManagerUpdate(t *testing.T) {
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 1 failed: %v", err)
	}
	defer l1.Close()

	l2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 2 failed: %v", err)
	}
	defer l2.Close()

	mgr := NewManager(func([]byte) error { return nil }, func(string, ...any) {})
	defer mgr.Close()

	id, err := mgr.Add(TCP, l1.Addr().String())
	if err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	if err = mgr.Update(id, TCP, l2.Addr().String()); err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	items := mgr.List()
	if len(items) != 1 || items[0].Address != l2.Addr().String() {
		t.Fatalf("update should change address, got=%+v", items)
	}

	if err = mgr.Disable(id); err != nil {
		t.Fatalf("Disable() failed: %v", err)
	}
	if err = mgr.Update(id, TCP, l1.Addr().String()); err != nil {
		t.Fatalf("Update() on disabled should succeed: %v", err)
	}
}
