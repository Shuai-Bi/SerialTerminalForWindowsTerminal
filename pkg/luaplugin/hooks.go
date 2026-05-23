package luaplugin

import lua "github.com/yuin/gopher-lua"

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
