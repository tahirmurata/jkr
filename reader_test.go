/* Any copyright is dedicated to the Public Domain.
 * https://creativecommons.org/publicdomain/zero/1.0/ */

package jkr

import (
	"bytes"
	"compress/flate"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestReader_Read(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		setup   func(*lua.LState) *lua.LTable
		wantErr bool
	}{
		{
			name:  "empty table",
			input: "return {}",
			setup: func(L *lua.LState) *lua.LTable {
				return L.NewTable()
			},
			wantErr: false,
		},
		{
			name:  "string value",
			input: `return {['foo']="bar",}`,
			setup: func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", lua.LString("bar"))
				return tbl
			},
			wantErr: false,
		},
		{
			name:  "number value",
			input: `return {[1]=42,}`,
			setup: func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetInt(1, lua.LNumber(42))
				return tbl
			},
			wantErr: false,
		},
		{
			name:  "boolean value",
			input: `return {['flag']=true,}`,
			setup: func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("flag", lua.LBool(true))
				return tbl
			},
			wantErr: false,
		},
		{
			name:  "nested table",
			input: `return {['nested']={['a']=1,['b']=2,},}`,
			setup: func(L *lua.LState) *lua.LTable {
				nested := L.NewTable()
				nested.RawSetString("a", lua.LNumber(1))
				nested.RawSetString("b", lua.LNumber(2))
				tbl := L.NewTable()
				tbl.RawSetString("nested", nested)
				return tbl
			},
			wantErr: false,
		},
		{
			name:    "invalid lua",
			input:   "not a valid lua",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			zw, err := flate.NewWriter(&buf, flate.BestSpeed)
			if err != nil {
				t.Fatalf("flate.NewWriter error: %v", err)
			}
			_, _ = zw.Write([]byte(tc.input))
			zw.Close()

			r := NewReader(&buf)
			tbl, err := r.Read()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Read() error: %v", err)
			}
			if tbl == nil {
				t.Fatalf("Read() returned nil table")
			}

			// Compare with expected table using Lua VM
			if tc.setup != nil {
				L := lua.NewState()
				defer L.Close()
				expected := tc.setup(L)

				if !deepEquals(L, tbl, expected) {
					t.Errorf("tables not equal")
				}
			}
		})
	}
}

// deepEquals compares two lua tables using Lua's native comparison
func deepEquals(L *lua.LState, a, b *lua.LTable) bool {
	// Define the deep_equals function in Lua
	luaCode := `
		local function deep_equals(o1, o2, ignore_mt)
			-- same object
			if o1 == o2 then return true end

			local o1Type = type(o1)
			local o2Type = type(o2)
			--- different type
			if o1Type ~= o2Type then return false end
			--- same type but not table, already compared above
			if o1Type ~= 'table' then return false end

			-- use metatable method
			if not ignore_mt then
				local mt1 = getmetatable(o1)
				if mt1 and mt1.__eq then
					--compare using built in method
					return o1 == o2
				end
			end

			-- iterate over o1
			for key1, value1 in pairs(o1) do
				local value2 = o2[key1]
				if value2 == nil or deep_equals(value1, value2, ignore_mt) == false then
					return false
				end
			end

			--- check keys in o2 but missing from o1
			for key2, _ in pairs(o2) do
				if o1[key2] == nil then return false end
			end
			return true
		end
		
		return deep_equals
	`

	if err := L.DoString(luaCode); err != nil {
		return false
	}

	deepEqualsFn := L.Get(-1)
	L.Pop(1)

	// Call the Lua function with our tables
	L.Push(deepEqualsFn)
	L.Push(a)
	L.Push(b)
	L.Push(lua.LBool(true)) // ignore_mt = true

	err := L.PCall(3, 1, nil)
	if err != nil {
		return false
	}

	result := L.ToBool(-1)
	L.Pop(1)

	return result
}
