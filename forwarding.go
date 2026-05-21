package main

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type ForwardStats struct {
	ReadBytes    uint64
	WrittenBytes uint64
	LastError    string
}

type ForwardTarget struct {
	ID        int
	Mode      FoeWardMode
	Address   string
	Enabled   bool
	Connected bool
	CreatedAt time.Time

	conn    net.Conn
	stats   ForwardStats
	mu      sync.Mutex
	closeCh chan struct{}
	closed  bool
}

type ForwardSnapshot struct {
	ID        int
	Mode      string
	Address   string
	Enabled   bool
	Connected bool
	ReadBytes uint64
	WriteByte uint64
	LastError string
}

type ForwardManager struct {
	mu            sync.RWMutex
	targets       map[int]*ForwardTarget
	nextID        int
	writeToSerial func([]byte) error
	notify        func(string, ...any)
	onInbound     func(int, []byte)
}

func NewForwardManager(writeToSerial func([]byte) error, notify func(string, ...any)) *ForwardManager {
	return &ForwardManager{
		targets:       make(map[int]*ForwardTarget),
		nextID:        1,
		writeToSerial: writeToSerial,
		notify:        notify,
	}
}

func (m *ForwardManager) SetInboundReporter(fn func(int, []byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onInbound = fn
}

func (m *ForwardManager) Add(mode FoeWardMode, address string) (int, error) {
	if mode == NOT {
		return 0, fmt.Errorf("forward mode cannot be none")
	}

	t := &ForwardTarget{
		Mode:      mode,
		Address:   address,
		Enabled:   true,
		CreatedAt: time.Now(),
		closeCh:   make(chan struct{}),
	}

	conn, err := net.Dial(mode.Network(), address)
	if err != nil {
		t.stats.LastError = err.Error()
		return 0, err
	}

	t.conn = conn
	t.Connected = true

	m.mu.Lock()
	t.ID = m.nextID
	m.nextID++
	m.targets[t.ID] = t
	m.mu.Unlock()

	go m.readLoop(t, conn, t.closeCh)
	m.notify("[forward] #%d %s %s connected", t.ID, t.Mode.String(), t.Address)
	return t.ID, nil
}

func (m *ForwardManager) readLoop(t *ForwardTarget, conn net.Conn, stop <-chan struct{}) {
	buf := make([]byte, 4096)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			atomic.AddUint64(&t.stats.ReadBytes, uint64(n))
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			if wErr := m.writeToSerial(chunk); wErr != nil {
				t.stats.LastError = wErr.Error()
				m.notify("[forward] #%d write serial error: %v", t.ID, wErr)
			} else if m.onInbound != nil {
				m.onInbound(t.ID, chunk)
			}
		}

		if err != nil {
			t.mu.Lock()
			if t.conn == conn {
				t.Connected = false
			}
			t.stats.LastError = err.Error()
			t.mu.Unlock()
			m.notify("[forward] #%d disconnected: %v", t.ID, err)
			return
		}

		select {
		case <-stop:
			return
		default:
		}
	}
}

func (m *ForwardManager) Remove(id int) error {
	m.mu.Lock()
	t, ok := m.targets[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("forward #%d not found", id)
	}
	delete(m.targets, id)
	m.mu.Unlock()

	t.close()
	m.notify("[forward] #%d removed", id)
	return nil
}

func (m *ForwardManager) Enable(id int) error {
	m.mu.RLock()
	t, ok := m.targets[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("forward #%d not found", id)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Enabled && t.Connected {
		return nil
	}

	conn, err := net.Dial(t.Mode.Network(), t.Address)
	if err != nil {
		t.stats.LastError = err.Error()
		return err
	}

	t.Enabled = true
	t.Connected = true
	t.conn = conn
	t.closeCh = make(chan struct{})
	t.closed = false
	go m.readLoop(t, conn, t.closeCh)
	m.notify("[forward] #%d enabled", id)
	return nil
}

func (m *ForwardManager) Update(id int, mode FoeWardMode, address string) error {
	if mode == NOT {
		return fmt.Errorf("forward mode cannot be none")
	}

	m.mu.RLock()
	t, ok := m.targets[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("forward #%d not found", id)
	}

	t.mu.Lock()
	wasEnabled := t.Enabled
	t.Mode = mode
	t.Address = address
	t.mu.Unlock()

	// Restart the target to apply new mode/address when enabled.
	t.close()

	if !wasEnabled {
		m.notify("[forward] #%d updated (disabled)", id)
		return nil
	}

	return m.Enable(id)
}

func (m *ForwardManager) Disable(id int) error {
	m.mu.RLock()
	t, ok := m.targets[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("forward #%d not found", id)
	}

	t.mu.Lock()
	t.Enabled = false
	t.mu.Unlock()
	t.close()
	m.notify("[forward] #%d disabled", id)
	return nil
}

func (m *ForwardManager) Broadcast(data []byte) {
	if len(data) == 0 {
		return
	}

	m.mu.RLock()
	items := make([]*ForwardTarget, 0, len(m.targets))
	for _, t := range m.targets {
		items = append(items, t)
	}
	m.mu.RUnlock()

	for _, t := range items {
		if !t.Enabled || !t.Connected || t.conn == nil {
			continue
		}

		n, err := t.conn.Write(data)
		if err != nil {
			t.stats.LastError = err.Error()
			m.notify("[forward] #%d write error: %v", t.ID, err)
			continue
		}

		atomic.AddUint64(&t.stats.WrittenBytes, uint64(n))
	}
}

func (m *ForwardManager) List() []ForwardSnapshot {
	m.mu.RLock()
	items := make([]ForwardSnapshot, 0, len(m.targets))
	for _, t := range m.targets {
		items = append(items, ForwardSnapshot{
			ID:        t.ID,
			Mode:      t.Mode.String(),
			Address:   t.Address,
			Enabled:   t.Enabled,
			Connected: t.Connected,
			ReadBytes: atomic.LoadUint64(&t.stats.ReadBytes),
			WriteByte: atomic.LoadUint64(&t.stats.WrittenBytes),
			LastError: t.stats.LastError,
		})
	}
	m.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	return items
}

func (m *ForwardManager) Close() {
	m.mu.Lock()
	items := make([]*ForwardTarget, 0, len(m.targets))
	for _, t := range m.targets {
		items = append(items, t)
	}
	m.targets = map[int]*ForwardTarget{}
	m.mu.Unlock()

	for _, t := range items {
		t.close()
	}
}

func (t *ForwardTarget) close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	ch := t.closeCh
	conn := t.conn
	t.conn = nil
	t.Connected = false
	t.mu.Unlock()

	if ch != nil {
		close(ch)
	}
	if conn != nil {
		_ = conn.Close()
	}
}
