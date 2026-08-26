package lua

import (
	"fmt"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// string library

func luaStringByte(L *lua.LState) int {
	s := []rune(L.CheckString(1))
	start := int(L.OptNumber(2, 1))
	if start < 1 {
		start = 1
	}
	end := int(L.OptNumber(3, lua.LNumber(start)))
	if end > len(s) {
		end = len(s)
	}
	if start > len(s) {
		return 0
	}
	for i := start; i <= end; i++ {
		L.Push(lua.LNumber(s[i-1]))
	}
	return end - start + 1
}

func luaStringChar(L *lua.LState) int {
	n := int(L.CheckNumber(1))
	L.Push(lua.LString(string(rune(n))))
	return 1
}

func luaStringFind(L *lua.LState) int {
	s := L.CheckString(1)
	pattern := L.CheckString(2)
	start := int(L.OptNumber(3, 1))
	idx := strings.Index(s[start-1:], pattern)
	if idx == -1 {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LNumber(start + idx))
	L.Push(lua.LNumber(start + idx + len(pattern) - 1))
	return 2
}

func luaStringFormat(L *lua.LState) int {
	fmt_str := L.CheckString(1)
	args := make([]any, 0)
	for i := 2; i <= L.GetTop(); i++ {
		args = append(args, L.CheckAny(i))
	}
	L.Push(lua.LString(fmt.Sprintf(fmt_str, args...)))
	return 1
}

func luaStringGsub(L *lua.LState) int {
	s := L.CheckString(1)
	old := L.CheckString(2)
	new := L.CheckString(3)
	L.Push(lua.LString(strings.ReplaceAll(s, old, new)))
	return 1
}

func luaStringLen(L *lua.LState) int {
	L.Push(lua.LNumber(len(L.CheckString(1))))
	return 1
}

func luaStringLower(L *lua.LState) int {
	L.Push(lua.LString(strings.ToLower(L.CheckString(1))))
	return 1
}

func luaStringUpper(L *lua.LState) int {
	L.Push(lua.LString(strings.ToUpper(L.CheckString(1))))
	return 1
}

func luaStringTrim(L *lua.LState) int {
	L.Push(lua.LString(strings.TrimSpace(L.CheckString(1))))
	return 1
}

func luaStringMatch(L *lua.LState) int {
	s := L.CheckString(1)
	pattern := L.CheckString(2)
	idx := strings.Index(s, pattern)
	if idx == -1 {
		L.Push(lua.LNil)
		return 1
	}
	L.Push(lua.LString(s[idx : idx+len(pattern)]))
	return 1
}

func luaStringRep(L *lua.LState) int {
	s := L.CheckString(1)
	n := int(L.CheckNumber(2))
	L.Push(lua.LString(strings.Repeat(s, n)))
	return 1
}

func luaStringSub(L *lua.LState) int {
	s := L.CheckString(1)
	start := int(L.CheckNumber(2))
	end := int(L.OptNumber(3, lua.LNumber(len(s))))
	if start < 1 {
		start = 1
	}
	if end > len(s) {
		end = len(s)
	}
	if start > end {
		L.Push(lua.LString(""))
		return 1
	}
	L.Push(lua.LString(s[start-1 : end]))
	return 1
}

// math library

func luaMathAbs(L *lua.LState) int {
	n := L.CheckNumber(1)
	if n < 0 {
		n = -n
	}
	L.Push(n)
	return 1
}

func luaMathCeil(L *lua.LState) int {
	n := L.CheckNumber(1)
	L.Push(lua.LNumber(int(n)))
	return 1
}

func luaMathFloor(L *lua.LState) int {
	n := L.CheckNumber(1)
	L.Push(lua.LNumber(int(n)))
	return 1
}

func luaMathMax(L *lua.LState) int {
	max := L.CheckNumber(1)
	for i := 2; i <= L.GetTop(); i++ {
		n := L.CheckNumber(i)
		if n > max {
			max = n
		}
	}
	L.Push(max)
	return 1
}

func luaMathMin(L *lua.LState) int {
	min := L.CheckNumber(1)
	for i := 2; i <= L.GetTop(); i++ {
		n := L.CheckNumber(i)
		if n < min {
			min = n
		}
	}
	L.Push(min)
	return 1
}

func luaMathSqrt(L *lua.LState) int {
	n := L.CheckNumber(1)
	L.Push(lua.LNumber(float64(n)))
	return 1
}

// table library

func luaTableMaxN(L *lua.LState) int {
	tbl := L.CheckTable(1)
	return tbl.MaxN()
}

func luaTableInsert(L *lua.LState) int {
	tbl := L.CheckTable(1)
	var pos int
	var val lua.LValue
	if L.GetTop() == 2 {
		pos = tbl.MaxN() + 1
		val = L.Get(2)
	} else {
		pos = int(L.CheckNumber(2))
		val = L.CheckAny(3)
	}
	tbl.Insert(pos, val)
	return 0
}

func luaTableRemove(L *lua.LState) int {
	tbl := L.CheckTable(1)
	pos := int(L.OptNumber(2, lua.LNumber(tbl.MaxN())))
	val := tbl.Remove(pos)
	if val != nil {
		L.Push(val)
	} else {
		L.Push(lua.LNil)
	}
	return 1
}

func luaTableSort(L *lua.LState) int {
	tbl := L.CheckTable(1)
	length := tbl.MaxN()
	keys := make([]lua.LValue, length)
	for i := 1; i <= length; i++ {
		keys[i-1] = tbl.RawGetInt(i)
	}
	if L.GetTop() >= 2 {
		fn := L.CheckFunction(2)
		for i := 1; i < length; i++ {
			for j := i; j > 0; j-- {
				L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, keys[j-1], keys[j])
				if L.Get(-1).(lua.LBool) {
					keys[j-1], keys[j] = keys[j], keys[j-1]
				}
				L.Pop(1)
			}
		}
	} else {
		for i := 1; i < length; i++ {
			for j := i; j > 0; j-- {
				a := fmt.Sprintf("%v", keys[j-1])
				b := fmt.Sprintf("%v", keys[j])
				if a > b {
					keys[j-1], keys[j] = keys[j], keys[j-1]
				}
			}
		}
	}
	for i, v := range keys {
		tbl.RawSetInt(i+1, v)
	}
	return 0
}

func luaTableConcat(L *lua.LState) int {
	tbl := L.CheckTable(1)
	sep := L.OptString(2, "")
	length := tbl.MaxN()
	var sb strings.Builder
	for i := 1; i <= length; i++ {
		if i > 1 {
			sb.WriteString(sep)
		}
		sb.WriteString(fmt.Sprintf("%v", tbl.RawGetInt(i)))
	}
	L.Push(lua.LString(sb.String()))
	return 1
}

func luaTableKeys(L *lua.LState) int {
	tbl := L.CheckTable(1)
	keys := L.NewTable()
	idx := 1
	tbl.ForEach(func(_ lua.LValue, v lua.LValue) {
		keys.RawSetInt(idx, v)
		idx++
	})
	L.Push(keys)
	return 1
}

// type system

func luaType(L *lua.LState) int {
	v := L.CheckAny(1)
	L.Push(lua.LString(v.Type().String()))
	return 1
}

func luaTostring(L *lua.LState) int {
	v := L.CheckAny(1)
	L.Push(lua.LString(v.String()))
	return 1
}

func luaTonumber(L *lua.LState) int {
	v := L.CheckAny(1)
	switch val := v.(type) {
	case lua.LNumber:
		L.Push(val)
	case lua.LString:
		n, err := strconv.ParseFloat(string(val), 64)
		if err != nil {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LNumber(n))
		}
	default:
		L.Push(lua.LNil)
	}
	return 1
}

func luaPrint(L *lua.LState) int {
	args := make([]any, L.GetTop())
	for i := 1; i <= L.GetTop(); i++ {
		args[i-1] = L.CheckAny(i).String()
	}
	fmt.Println(args...)
	return 0
}
