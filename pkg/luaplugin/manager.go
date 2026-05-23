// Package luaplugin provides a Lua plugin system for processing serial data streams.
package luaplugin

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

// Plugin represents a loaded Lua plugin.
type Plugin struct {
	Name    string
	Path    string
	Enabled bool
	L       *lua.LState
	callMu  sync.Mutex
}

// Snapshot is a read-only view of a plugin for display.
type Snapshot struct {
	Name    string
	Path    string
	Enabled bool
}

// Manager coordinates plugin lifecycle and hook execution.
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin
}

// NewManager creates a plugin manager.
func NewManager() *Manager {
	return &Manager{plugins: make(map[string]*Plugin)}
}

// Load loads a Lua plugin from the given path.
func (m *Manager) Load(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	name := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	if name == "" {
		return "", fmt.Errorf("invalid plugin name")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.plugins[name]; ok {
		return "", fmt.Errorf("plugin %s already loaded", name)
	}

	state := lua.NewState()
	registerHelpers(state)
	if err = state.DoFile(abs); err != nil {
		state.Close()
		return "", err
	}

	m.plugins[name] = &Plugin{
		Name:    name,
		Path:    abs,
		Enabled: true,
		L:       state,
	}

	return name, nil
}

// Unload unloads a plugin and closes its Lua state.
func (m *Manager) Unload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}

	p.L.Close()
	delete(m.plugins, name)
	return nil
}

// Enable enables a previously loaded plugin.
func (m *Manager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	p.Enabled = true
	return nil
}

// Disable disables a plugin without unloading it.
func (m *Manager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	p.Enabled = false
	return nil
}

// Reload reloads a plugin's file atomically.
func (m *Manager) Reload(name string) error {
	m.mu.Lock()
	p, ok := m.plugins[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("plugin %s not found", name)
	}

	path := p.Path
	p.L.Close()
	delete(m.plugins, name)
	m.mu.Unlock()

	_, err := m.Load(path)
	return err
}

// List returns a snapshot of all plugins.
func (m *Manager) List() []Snapshot {
	m.mu.RLock()
	res := make([]Snapshot, 0, len(m.plugins))
	for _, p := range m.plugins {
		res = append(res, Snapshot{Name: p.Name, Path: p.Path, Enabled: p.Enabled})
	}
	m.mu.RUnlock()

	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	return res
}

// ProcessInput runs the OnInput hook chain across all enabled plugins.
func (m *Manager) ProcessInput(data []byte) ([]byte, error) {
	return m.processDataHook("OnInput", data)
}

// ProcessOutput runs the OnOutput hook chain across all enabled plugins.
func (m *Manager) ProcessOutput(data []byte) ([]byte, error) {
	return m.processDataHook("OnOutput", data)
}

func (m *Manager) processDataHook(name string, data []byte) ([]byte, error) {
	m.mu.RLock()
	plugins := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	m.mu.RUnlock()

	current := data
	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		p.callMu.Lock()
		ret, called, err := callStringHook(p.L, name, string(current))
		p.callMu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("plugin %s %s: %w", p.Name, name, err)
		}
		if !called {
			continue
		}
		if ret == nil {
			return nil, nil
		}
		current = []byte(*ret)
	}

	return current, nil
}

// ProcessCommand runs the OnCommand hook chain across all enabled plugins.
func (m *Manager) ProcessCommand(line string) (string, bool, error) {
	m.mu.RLock()
	plugins := make([]*Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	m.mu.RUnlock()

	current := line
	allow := true
	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		p.callMu.Lock()
		next, nextAllow, called, err := callCommandHook(p.L, "OnCommand", current)
		p.callMu.Unlock()
		if err != nil {
			return "", false, fmt.Errorf("plugin %s OnCommand: %w", p.Name, err)
		}
		if !called {
			continue
		}
		allow = allow && nextAllow
		if !allow {
			return "", false, nil
		}
		if next != "" {
			current = next
		}
	}

	return current, true, nil
}

// Close closes all plugin Lua states.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plugins {
		p.L.Close()
	}
	m.plugins = map[string]*Plugin{}
}
