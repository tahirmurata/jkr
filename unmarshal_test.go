/* Any copyright is dedicated to the Public Domain.
 * https://creativecommons.org/publicdomain/zero/1.0/ */

package jkr

import (
	"bytes"
	"compress/flate"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		lua       string
		expected  func(*lua.LState) *lua.LTable
		expectErr bool
	}{
		{
			"empty table",
			`return {}`,
			func(L *lua.LState) *lua.LTable {
				return L.NewTable()
			}, false},
		{
			"string value",
			`return {["foo"]="bar",}`,
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", lua.LString("bar"))
				return tbl
			}, false},
		{
			"number key and value",
			`return {[1]=42,}`,
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetInt(1, lua.LNumber(42))
				return tbl
			}, false},
		{
			"boolean value",
			`return {["foo"]=true,["bar"]=false,}`,
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", lua.LBool(true))
				tbl.RawSetString("bar", lua.LBool(false))
				return tbl
			}, false},
		{
			"nested table",
			`return {["nested"]={["a"]=1,["b"]=2,},}`,
			func(L *lua.LState) *lua.LTable {
				nested := L.NewTable()
				nested.RawSetString("a", lua.LNumber(1))
				nested.RawSetString("b", lua.LNumber(2))
				tbl := L.NewTable()
				tbl.RawSetString("nested", nested)
				return tbl
			}, false},
		{
			"not returning table",
			`return "foo"`,
			func(L *lua.LState) *lua.LTable {
				return L.NewTable()
			}, true},
		{
			"not returning valid lua code",
			`@`,
			func(L *lua.LState) *lua.LTable {
				return L.NewTable()
			}, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			L := lua.NewState()
			defer L.Close()

			want := test.expected(L)

			// compress the Lua code
			var buf bytes.Buffer
			w, err := flate.NewWriter(&buf, flate.DefaultCompression)
			if err != nil {
				t.Fatalf("failed to create flate writer for %q: %v", test.name, err)
			}
			if _, err := w.Write([]byte(test.lua)); err != nil {
				t.Fatalf("failed to write data for %q: %v", test.name, err)
			}
			w.Close()
			data := buf.Bytes()

			var out lua.LTable
			err = Unmarshal(data, &out)
			if test.expectErr {
				if err == nil {
					t.Fatalf("expected error for test %q, got nil", test.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal() error for %q: %v", test.name, err)
			}

			if !deepEquals(L, want, &out) {
				t.Errorf("failed to unmarshal %q: tables not equal", test.name)
			}
		})
	}
}

// deepEquals compares two lua tables for deep equality using Lua code
func deepEquals(L *lua.LState, a, b *lua.LTable) bool {
	luaCode := `
        local function deep_equals(o1, o2)
            if o1 == o2 then return true end
            if type(o1) ~= type(o2) then return false end
            if type(o1) ~= 'table' then return false end
            for k, v in pairs(o1) do
                if not deep_equals(v, o2[k]) then return false end
            end
            for k in pairs(o2) do
                if o1[k] == nil then return false end
            end
            return true
        end
        return deep_equals
    `
	if err := L.DoString(luaCode); err != nil {
		return false
	}
	fn := L.Get(-1)
	L.Pop(1)
	L.Push(fn)
	L.Push(a)
	L.Push(b)
	if err := L.PCall(2, 1, nil); err != nil {
		return false
	}
	res := L.ToBool(-1)
	L.Pop(1)
	return res
}
