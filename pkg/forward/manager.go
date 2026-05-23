// Package forward manages TCP/UDP forwarding targets for serial data.
package forward

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Mode is the forwarding protocol mode.
type Mode int

const (
	None Mode = iota
	TCP
	UDP
)

// ParseMode parses a mode string. Accepts "tcp"/"tcp-c"/"tcpc"/"1" → TCP, "udp"/"udp-c"/"udpc"/"2" → UDP.
func ParseMode(v string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "tcp", "tcp-c", "tcpc", "1":
		return TCP, true
	case "udp", "udp-c", "udpc", "2":
		return UDP, true
	default:
		return None, false
	}
}

func (m Mode) Network() string {
	switch m {
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	default:
		return ""
	}
}

func (m Mode) String() string {
	switch m {
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	default:
		return "none"
	}
}

// Stats holds I/O statistics for a forward target.
type Stats struct {
	ReadBytes    uint64
	WrittenBytes uint64
	LastError    string
}

// Target represents a single forwarding connection.
type Target struct {
	ID        int
	Mode      Mode
	Address   string
	Enabled   bool
	Connected bool
	CreatedAt time.Time

	conn    net.Conn
	stats   Stats
	mu      sync.Mutex
	closeCh chan struct{}
	closed  bool
}

// Snapshot is a read-only view of a forward target for display.
type Snapshot struct {
	ID        int
	Mode      string
	Address   string
	Enabled   bool
	Connected bool
	ReadBytes uint64
	WriteByte uint64
	LastError string
}

// Manager coordinates forwarding targets.
type Manager struct {
	mu            sync.RWMutex
	targets       map[int]*Target
	nextID        int
	writeToSerial func([]byte) error
	notify        func(string, ...any)
	onInbound     func(int, []byte)
}

// NewManager creates a forwarding manager.
func NewManager(writeToSerial func([]byte) error, notify func(string, ...any)) *Manager {
	return &Manager{
		targets:       make(map[int]*Target),
		nextID:        1,
		writeToSerial: writeToSerial,
		notify:        notify,
	}
}

// SetInboundReporter sets a callback invoked when inbound data arrives from a target.
func (m *Manager) SetInboundReporter(fn func(int, []byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onInbound = fn
}

// Add creates and connects a new forward target.
func (m *Manager) Add(mode Mode, address string) (int, error) {
	if mode == None {
		return 0, fmt.Errorf("forward mode cannot be none")
	}

	t := &Target{
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

func (m *Manager) readLoop(t *Target, conn net.Conn, stop <-chan struct{}) {
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

// Remove disconnects and removes a target.
func (m *Manager) Remove(id int) error {
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

// Enable (re)connects a target.
func (m *Manager) Enable(id int) error {
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

// Update changes a target's mode and address, reconnecting if enabled.
func (m *Manager) Update(id int, mode Mode, address string) error {
	if mode == None {
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

	t.close()

	if !wasEnabled {
		m.notify("[forward] #%d updated (disabled)", id)
		return nil
	}

	return m.Enable(id)
}

// Disable disconnects a target without removing it.
func (m *Manager) Disable(id int) error {
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

// Broadcast sends data to all enabled, connected targets.
func (m *Manager) Broadcast(data []byte) {
	if len(data) == 0 {
		return
	}

	m.mu.RLock()
	items := make([]*Target, 0, len(m.targets))
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

// List returns a snapshot of all targets.
func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	items := make([]Snapshot, 0, len(m.targets))
	for _, t := range m.targets {
		items = append(items, Snapshot{
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

// Close disconnects and removes all targets.
func (m *Manager) Close() {
	m.mu.Lock()
	items := make([]*Target, 0, len(m.targets))
	for _, t := range m.targets {
		items = append(items, t)
	}
	m.targets = map[int]*Target{}
	m.mu.Unlock()

	for _, t := range items {
		t.close()
	}
}

func (t *Target) close() {
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
