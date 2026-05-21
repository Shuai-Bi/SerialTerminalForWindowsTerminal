package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	lua "github.com/yuin/gopher-lua"
)

type LuaPlugin struct {
	Name    string
	Path    string
	Enabled bool
	L       *lua.LState
	callMu  sync.Mutex
}

type PluginSnapshot struct {
	Name    string
	Path    string
	Enabled bool
}

type PluginManager struct {
	mu      sync.RWMutex
	plugins map[string]*LuaPlugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{plugins: make(map[string]*LuaPlugin)}
}

func (m *PluginManager) Load(path string) (string, error) {
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
	if err = state.DoFile(abs); err != nil {
		state.Close()
		return "", err
	}

	m.plugins[name] = &LuaPlugin{
		Name:    name,
		Path:    abs,
		Enabled: true,
		L:       state,
	}

	return name, nil
}

func (m *PluginManager) Unload(name string) error {
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

func (m *PluginManager) Enable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	p.Enabled = true
	return nil
}

func (m *PluginManager) Disable(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}
	p.Enabled = false
	return nil
}

func (m *PluginManager) Reload(name string) error {
	m.mu.Lock()
	p, ok := m.plugins[name]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("plugin %s not found", name)
	}

	path := p.Path
	if err := m.Unload(name); err != nil {
		return err
	}
	_, err := m.Load(path)
	return err
}

func (m *PluginManager) List() []PluginSnapshot {
	m.mu.RLock()
	res := make([]PluginSnapshot, 0, len(m.plugins))
	for _, p := range m.plugins {
		res = append(res, PluginSnapshot{Name: p.Name, Path: p.Path, Enabled: p.Enabled})
	}
	m.mu.RUnlock()

	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})
	return res
}

func (m *PluginManager) ProcessInput(data []byte) ([]byte, error) {
	return m.processDataHook("OnInput", data)
}

func (m *PluginManager) ProcessOutput(data []byte) ([]byte, error) {
	return m.processDataHook("OnOutput", data)
}

func (m *PluginManager) processDataHook(name string, data []byte) ([]byte, error) {
	m.mu.RLock()
	plugins := make([]*LuaPlugin, 0, len(m.plugins))
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

func (m *PluginManager) ProcessCommand(line string) (string, bool, error) {
	m.mu.RLock()
	plugins := make([]*LuaPlugin, 0, len(m.plugins))
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

func (m *PluginManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.plugins {
		p.L.Close()
	}
	m.plugins = map[string]*LuaPlugin{}
}

func callStringHook(L *lua.LState, name string, payload string) (*string, bool, error) {
	fn := L.GetGlobal(name)
	if fn.Type() == lua.LTNil {
		return nil, false, nil
	}

	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, lua.LString(payload)); err != nil {
		return nil, true, err
	}

	ret := L.Get(-1)
	L.Pop(1)
	if ret.Type() == lua.LTNil {
		return nil, true, nil
	}

	s := ret.String()
	return &s, true, nil
}

func callCommandHook(L *lua.LState, name, line string) (string, bool, bool, error) {
	fn := L.GetGlobal(name)
	if fn.Type() == lua.LTNil {
		return "", true, false, nil
	}

	if err := L.CallByParam(lua.P{Fn: fn, NRet: 2, Protect: true}, lua.LString(line)); err != nil {
		return "", true, true, err
	}

	allowVal := L.Get(-1)
	lineVal := L.Get(-2)
	L.Pop(2)

	allow := true
	if allowVal.Type() == lua.LTBool {
		allow = lua.LVAsBool(allowVal)
	}

	next := ""
	if lineVal.Type() != lua.LTNil {
		next = lineVal.String()
	}

	return next, allow, true, nil
}
