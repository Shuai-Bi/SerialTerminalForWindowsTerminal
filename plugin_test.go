package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLuaScript(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write lua script failed: %v", err)
	}
	return path
}

func TestPluginManagerLoadAndHooks(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "rewrite.lua", `
function OnInput(s)
  return s .. "-in"
end

function OnOutput(s)
  return s .. "-out"
end

function OnCommand(line)
  return line .. " --lua", true
end
`)

	name, err := m.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if name != "rewrite" {
		t.Fatalf("unexpected plugin name: %q", name)
	}

	in, err := m.ProcessInput([]byte("abc"))
	if err != nil {
		t.Fatalf("ProcessInput() failed: %v", err)
	}
	if string(in) != "abc-in" {
		t.Fatalf("ProcessInput() got=%q want=%q", in, "abc-in")
	}

	out, err := m.ProcessOutput([]byte("xyz"))
	if err != nil {
		t.Fatalf("ProcessOutput() failed: %v", err)
	}
	if string(out) != "xyz-out" {
		t.Fatalf("ProcessOutput() got=%q want=%q", out, "xyz-out")
	}

	line, allow, err := m.ProcessCommand(".help")
	if err != nil {
		t.Fatalf("ProcessCommand() failed: %v", err)
	}
	if !allow || line != ".help --lua" {
		t.Fatalf("ProcessCommand() got=(%q,%v) want=(%q,true)", line, allow, ".help --lua")
	}
}

func TestPluginManagerDisableAndUnload(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "simple.lua", `
function OnInput(s)
  return s .. "-x"
end
`)

	name, err := m.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if err = m.Disable(name); err != nil {
		t.Fatalf("Disable() failed: %v", err)
	}
	got, err := m.ProcessInput([]byte("abc"))
	if err != nil {
		t.Fatalf("ProcessInput() with disabled plugin failed: %v", err)
	}
	if string(got) != "abc" {
		t.Fatalf("disabled plugin should not modify input, got=%q", got)
	}

	if err = m.Enable(name); err != nil {
		t.Fatalf("Enable() failed: %v", err)
	}
	got, err = m.ProcessInput([]byte("abc"))
	if err != nil {
		t.Fatalf("ProcessInput() after enable failed: %v", err)
	}
	if string(got) != "abc-x" {
		t.Fatalf("enabled plugin should modify input, got=%q", got)
	}

	if err = m.Unload(name); err != nil {
		t.Fatalf("Unload() failed: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatalf("Unload() should remove plugin from list")
	}
}

func TestPluginManagerOutputDrop(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "drop.lua", `
function OnOutput(s)
  return nil
end
`)

	if _, err := m.Load(path); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	out, err := m.ProcessOutput([]byte("abc"))
	if err != nil {
		t.Fatalf("ProcessOutput() failed: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil output when plugin returns nil")
	}
}

func TestPluginManagerReload(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "reloadable.lua", `
function OnInput(s)
  return s .. "-v1"
end
`)
	name, err := m.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if err = m.Reload(name); err != nil {
		t.Fatalf("Reload() failed: %v", err)
	}

	out, err := m.ProcessInput([]byte("test"))
	if err != nil {
		t.Fatalf("ProcessInput() after reload failed: %v", err)
	}
	if string(out) != "test-v1" {
		t.Fatalf("reloaded plugin should still work, got=%q", out)
	}

	if err = m.Reload("nonexistent"); err == nil {
		t.Fatalf("Reload() non-existent should error")
	}
}

func TestPluginManagerCommandBlock(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "blocker.lua", `
function OnCommand(line)
  return line, false
end
`)

	if _, err := m.Load(path); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	line, allow, err := m.ProcessCommand(".exit")
	if err != nil {
		t.Fatalf("ProcessCommand() failed: %v", err)
	}
	if allow {
		t.Fatalf("command should be blocked, got allow=%v line=%q", allow, line)
	}
}

func TestPluginManagerLoadErrors(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	_, err := m.Load("nonexistent_file.lua")
	if err == nil {
		t.Fatalf("Load() non-existent file should error")
	}

	path := writeLuaScript(t, "bad.lua", "this is not valid lua {{{")
	_, err = m.Load(path)
	if err == nil {
		t.Fatalf("Load() invalid lua should error")
	}
}

func TestPluginManagerDuplicateLoad(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "once.lua", "function OnInput(s) return s end")
	_, err := m.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	_, err = m.Load(path)
	if err == nil {
		t.Fatalf("Load() duplicate should error")
	}
}

func TestPluginManagerListWithDisabled(t *testing.T) {
	m := NewPluginManager()
	t.Cleanup(m.Close)

	path := writeLuaScript(t, "mylist.lua", "function OnInput(s) return s end")
	name, err := m.Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if err = m.Disable(name); err != nil {
		t.Fatalf("Disable() failed: %v", err)
	}

	items := m.List()
	if len(items) != 1 || items[0].Enabled {
		t.Fatalf("expected disabled in list, got %+v", items)
	}
}
