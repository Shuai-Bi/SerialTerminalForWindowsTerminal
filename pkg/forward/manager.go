// Package forward manages TCP/UDP/COM forwarding targets for serial data.
package forward

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.bug.st/serial"
)

// Mode is the forwarding protocol mode.
type Mode int

const (
	None      Mode = 0
	TCP       Mode = 1
	UDP       Mode = 2
	TCPServer Mode = 3
	UDPServer Mode = 4
	COMPort   Mode = 5
)

// ParseMode parses a mode string.
func ParseMode(v string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "tcp", "tcp-c", "tcpc", "1":
		return TCP, true
	case "udp", "udp-c", "udpc", "2":
		return UDP, true
	case "tcp-s", "tcps", "tcp-server", "3":
		return TCPServer, true
	case "udp-s", "udps", "udp-server", "4":
		return UDPServer, true
	case "com", "serial", "5":
		return COMPort, true
	default:
		return None, false
	}
}

func (m Mode) Network() string {
	switch m {
	case TCP, TCPServer:
		return "tcp"
	case UDP, UDPServer:
		return "udp"
	case COMPort:
		return "serial"
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
	case TCPServer:
		return "tcp-s"
	case UDPServer:
		return "udp-s"
	case COMPort:
		return "com"
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

	// Client-mode connection (TCP/UDP client)
	conn net.Conn

	// Server-mode fields
	listener net.Listener              // TCP server listener
	conns    map[net.Conn]struct{}     // TCP server accepted connections
	connsMu  sync.Mutex

	// UDP server
	packetConn  net.PacketConn         // UDP server listener
	remoteAddrs map[string]net.Addr    // known UDP remotes

	// COM port
	serialPort serial.Port

	stats   Stats
	mu      sync.Mutex
	closeCh chan struct{}
	closed  bool
}

// AcceptedConns returns the number of accepted connections (TCP server only).
func (t *Target) acceptedConns() int {
	t.connsMu.Lock()
	defer t.connsMu.Unlock()
	return len(t.conns)
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
	Conns     int // accepted connection count (TCP server)
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

	switch mode {
	case TCP, UDP:
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

	case TCPServer:
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.stats.LastError = err.Error()
			return 0, err
		}
		t.listener = listener
		t.conns = make(map[net.Conn]struct{})
		t.Connected = true

		m.mu.Lock()
		t.ID = m.nextID
		m.nextID++
		m.targets[t.ID] = t
		m.mu.Unlock()

		go m.acceptLoop(t)

	case UDPServer:
		pc, err := net.ListenPacket("udp", address)
		if err != nil {
			t.stats.LastError = err.Error()
			return 0, err
		}
		t.packetConn = pc
		t.remoteAddrs = make(map[string]net.Addr)
		t.Connected = true

		m.mu.Lock()
		t.ID = m.nextID
		m.nextID++
		m.targets[t.ID] = t
		m.mu.Unlock()

		go m.readLoopPacket(t)

	case COMPort:
		sp, err := serial.Open(address, &serial.Mode{BaudRate: 115200, DataBits: 8, StopBits: 0, Parity: 0})
		if err != nil {
			t.stats.LastError = err.Error()
			return 0, err
		}
		t.serialPort = sp
		t.Connected = true

		m.mu.Lock()
		t.ID = m.nextID
		m.nextID++
		m.targets[t.ID] = t
		m.mu.Unlock()

		go m.readLoopSerial(t)
	}

	m.notify("[forward] #%d %s %s connected", t.ID, t.Mode.String(), t.Address)
	return t.ID, nil
}

func (m *Manager) acceptLoop(t *Target) {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			select {
			case <-t.closeCh:
				return
			default:
			}
			t.stats.LastError = err.Error()
			m.notify("[forward] #%d accept error: %v", t.ID, err)
			return
		}

		t.connsMu.Lock()
		t.conns[conn] = struct{}{}
		t.connsMu.Unlock()

		m.notify("[forward] #%d accepted %s", t.ID, conn.RemoteAddr())
		go m.readLoop(t, conn, t.closeCh)
	}
}

func (m *Manager) readLoopPacket(t *Target) {
	buf := make([]byte, 4096)
	for {
		n, addr, err := t.packetConn.ReadFrom(buf)
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
			// Track remote address for Broadcast
			t.mu.Lock()
			t.remoteAddrs[addr.String()] = addr
			t.mu.Unlock()
		}
		if err != nil {
			select {
			case <-t.closeCh:
				return
			default:
			}
			t.Connected = false
			t.stats.LastError = err.Error()
			m.notify("[forward] #%d disconnected: %v", t.ID, err)
			return
		}

		select {
		case <-t.closeCh:
			return
		default:
		}
	}
}

func (m *Manager) readLoopSerial(t *Target) {
	buf := make([]byte, 4096)
	for {
		n, err := t.serialPort.Read(buf)
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
			select {
			case <-t.closeCh:
				return
			default:
			}
			t.Connected = false
			t.stats.LastError = err.Error()
			m.notify("[forward] #%d disconnected: %v", t.ID, err)
			return
		}

		select {
		case <-t.closeCh:
			return
		default:
		}
	}
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
			t.Connected = false
			t.stats.LastError = err.Error()

			// Remove from TCP server conns if applicable
			if t.Mode == TCPServer {
				t.connsMu.Lock()
				delete(t.conns, conn)
				t.connsMu.Unlock()
			}
			m.notify("[forward] #%d disconnected: %v", t.ID, err)
			_ = conn.Close()
			return
		}

		select {
		case <-stop:
			_ = conn.Close()
			if t.Mode == TCPServer {
				t.connsMu.Lock()
				delete(t.conns, conn)
				t.connsMu.Unlock()
			}
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

	switch t.Mode {
	case TCP, UDP:
		conn, err := net.Dial(t.Mode.Network(), t.Address)
		if err != nil {
			t.stats.LastError = err.Error()
			return err
		}
		t.conn = conn
		t.Connected = true
		t.closeCh = make(chan struct{})
		t.closed = false
		go m.readLoop(t, conn, t.closeCh)

	case TCPServer:
		listener, err := net.Listen("tcp", t.Address)
		if err != nil {
			t.stats.LastError = err.Error()
			return err
		}
		t.listener = listener
		t.conns = make(map[net.Conn]struct{})
		t.Connected = true
		t.closeCh = make(chan struct{})
		t.closed = false
		go m.acceptLoop(t)

	case UDPServer:
		pc, err := net.ListenPacket("udp", t.Address)
		if err != nil {
			t.stats.LastError = err.Error()
			return err
		}
		t.packetConn = pc
		t.remoteAddrs = make(map[string]net.Addr)
		t.Connected = true
		t.closeCh = make(chan struct{})
		t.closed = false
		go m.readLoopPacket(t)

	case COMPort:
		sp, err := serial.Open(t.Address, &serial.Mode{BaudRate: 115200, DataBits: 8, StopBits: 0, Parity: 0})
		if err != nil {
			t.stats.LastError = err.Error()
			return err
		}
		t.serialPort = sp
		t.Connected = true
		t.closeCh = make(chan struct{})
		t.closed = false
		go m.readLoopSerial(t)
	}

	t.Enabled = true
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
		if !t.Enabled || !t.Connected {
			continue
		}

		switch t.Mode {
		case TCP, UDP:
			if t.conn == nil {
				continue
			}
			n, err := t.conn.Write(data)
			if err != nil {
				t.stats.LastError = err.Error()
				m.notify("[forward] #%d write error: %v", t.ID, err)
			} else {
				atomic.AddUint64(&t.stats.WrittenBytes, uint64(n))
			}

		case TCPServer:
			t.connsMu.Lock()
			conns := make([]net.Conn, 0, len(t.conns))
			for c := range t.conns {
				conns = append(conns, c)
			}
			t.connsMu.Unlock()
			for _, c := range conns {
				n, err := c.Write(data)
				if err != nil {
					t.stats.LastError = err.Error()
				} else {
					atomic.AddUint64(&t.stats.WrittenBytes, uint64(n))
				}
			}

		case UDPServer:
			t.mu.Lock()
			addrs := make([]net.Addr, 0, len(t.remoteAddrs))
			for _, addr := range t.remoteAddrs {
				addrs = append(addrs, addr)
			}
			t.mu.Unlock()
			for _, addr := range addrs {
				n, err := t.packetConn.WriteTo(data, addr)
				if err != nil {
					t.stats.LastError = err.Error()
				} else {
					atomic.AddUint64(&t.stats.WrittenBytes, uint64(n))
				}
			}

		case COMPort:
			if t.serialPort == nil {
				continue
			}
			n, err := t.serialPort.Write(data)
			if err != nil {
				t.stats.LastError = err.Error()
				m.notify("[forward] #%d write error: %v", t.ID, err)
			} else {
				atomic.AddUint64(&t.stats.WrittenBytes, uint64(n))
			}
		}
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
			Conns:     t.acceptedConns(),
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
	listener := t.listener
	pc := t.packetConn
	sp := t.serialPort
	t.conn = nil
	t.listener = nil
	t.packetConn = nil
	t.serialPort = nil
	t.Connected = false
	t.mu.Unlock()

	if ch != nil {
		close(ch)
	}
	if conn != nil {
		_ = conn.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if pc != nil {
		_ = pc.Close()
	}
	if sp != nil {
		_ = sp.Close()
	}
}
