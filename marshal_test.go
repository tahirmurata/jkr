/* Any copyright is dedicated to the Public Domain.
 * https://creativecommons.org/publicdomain/zero/1.0/ */

package jkr

import (
	"bytes"
	"compress/flate"
	"io"
	"testing"

	"slices"

	lua "github.com/yuin/gopher-lua"
)

func TestMarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(*lua.LState) *lua.LTable
		expected  []string
		expectErr bool
	}{
		{
			"empty table",
			func(L *lua.LState) *lua.LTable {
				return L.NewTable()
			}, []string{
				"return {}",
			}, false},
		{
			"string value",
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", lua.LString("bar"))
				return tbl
			}, []string{
				`return {["foo"]="bar",}`,
			}, false},
		{
			"number key and value",
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetInt(1, lua.LNumber(42))
				return tbl
			}, []string{
				`return {[1]=42,}`,
			}, false},
		{
			"boolean value",
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", lua.LBool(true))
				tbl.RawSetString("bar", lua.LBool(false))
				return tbl
			}, []string{
				`return {["foo"]=true,["bar"]=false,}`,
				`return {["bar"]=false,["foo"]=true,}`,
			}, false},
		{
			"nested table",
			func(L *lua.LState) *lua.LTable {
				nested := L.NewTable()
				nested.RawSetString("a", lua.LNumber(1))
				nested.RawSetString("b", lua.LNumber(2))
				tbl := L.NewTable()
				tbl.RawSetString("nested", nested)
				return tbl
			}, []string{
				`return {["nested"]={["a"]=1,["b"]=2,},}`,
				`return {["nested"]={["b"]=2,["a"]=1,},}`,
			}, false},
		{
			"circular reference",
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("self", lua.LNil) // will be set to itself later
				tbl.RawSetString("foo", lua.LString("bar"))
				tbl.RawSetString("self", tbl) // circular reference
				return tbl
			}, nil, true},
		{
			"invalid key type",
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSet(lua.LBool(true), lua.LString("invalid")) // boolean as key
				return tbl
			}, nil, true},
		{
			"object table with 'is' method",
			func(L *lua.LState) *lua.LTable {
				nested := L.NewTable()
				nested.RawSetString("is", L.NewFunction(func(L *lua.LState) int {
					return 0
				}))
				tbl := L.NewTable()
				tbl.RawSetString("foo", nested)
				return tbl
			}, []string{
				`return {["foo"]="MANUAL_REPLACE",}`,
			}, false},
		{
			"unsupported value type",
			func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", L.NewFunction(func(L *lua.LState) int {
					return 0
				}))
				return tbl
			}, nil, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			L := lua.NewState()
			defer L.Close()

			tbl := test.setup(L)
			data, err := Marshal(tbl)
			if test.expectErr {
				if err == nil {
					t.Fatalf("expected error for test %q, got nil", test.name)
				}
				return
			}
			if err != nil {
				t.Fatalf("Marshal() error for test %q: %v", test.name, err)
			}

			r := flate.NewReader(bytes.NewReader(data))
			defer r.Close()
			raw, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("ReadAll error: %v", err)
			}
			got := string(raw)
			found := slices.Contains(test.expected, got)
			if !found {
				t.Errorf("got %q; want one of %q", got, test.expected)
			}
		})
	}
}
