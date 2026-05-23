package luaplugin

import (
	lua "github.com/yuin/gopher-lua"
)

// registerHelpers registers Go utility functions into a Lua state.
func registerHelpers(L *lua.LState) {
	modbus := L.NewTable()
	L.SetGlobal("modbus", modbus)

	L.SetField(modbus, "crc16", L.NewFunction(luaCRC16))
	L.SetField(modbus, "validate", L.NewFunction(luaValidateCRC))

	hex := L.NewTable()
	L.SetGlobal("hex", hex)
	L.SetField(hex, "encode", L.NewFunction(luaHexEncode))
	L.SetField(hex, "decode", L.NewFunction(luaHexDecode))

	util := L.NewTable()
	L.SetGlobal("util", util)
	L.SetField(util, "bytes", L.NewFunction(luaBytes))
}

// crc16 computes the CRC-16/MODBUS checksum for the given data.
func crc16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func luaCRC16(L *lua.LState) int {
	s := L.CheckString(1)
	crc := crc16([]byte(s))
	L.Push(lua.LNumber(crc))
	return 1
}

func luaValidateCRC(L *lua.LState) int {
	s := L.CheckString(1)
	if len(s) < 2 {
		L.Push(lua.LBool(false))
		return 1
	}
	data := []byte(s[:len(s)-2])
	crc := crc16(data)
	expect := uint16(s[len(s)-2]) | uint16(s[len(s)-1])<<8
	L.Push(lua.LBool(crc == expect))
	return 1
}

func luaHexEncode(L *lua.LState) int {
	s := L.CheckString(1)
	buf := make([]byte, len(s)*2)
	for i, b := range []byte(s) {
		buf[i*2] = hexChar(b >> 4)
		buf[i*2+1] = hexChar(b & 0x0F)
	}
	L.Push(lua.LString(buf))
	return 1
}

func luaHexDecode(L *lua.LState) int {
	s := L.CheckString(1)
	if len(s)%2 != 0 {
		L.Push(lua.LNil)
		return 1
	}
	buf := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		buf[i/2] = unhexChar(s[i])<<4 | unhexChar(s[i+1])
	}
	L.Push(lua.LString(buf))
	return 1
}

func luaBytes(L *lua.LState) int {
	// Converts a sequence of numbers to a byte string.
	// e.g. util.bytes(0x01, 0x03, 0x00, 0x01, 0x00, 0x01) → "\x01\x03\x00\x01\x00\x01"
	top := L.GetTop()
	buf := make([]byte, top)
	for i := 1; i <= top; i++ {
		buf[i-1] = byte(L.CheckInt(i))
	}
	L.Push(lua.LString(buf))
	return 1
}

func hexChar(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'A' + (b - 10)
}

func unhexChar(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}
