/* Any copyright is dedicated to the Public Domain.
 * https://creativecommons.org/publicdomain/zero/1.0/ */

package jkr

import (
	"bytes"
	"compress/flate"
	"io"
	"slices"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestWriter_Write(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   func(*lua.LState) *lua.LTable
		want []string
	}{
		{
			name: "empty table",
			in: func(L *lua.LState) *lua.LTable {
				return L.NewTable()
			},
			want: []string{"return {}"},
		},
		{
			name: "simple string",
			in: func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("foo", lua.LString("bar"))
				return tbl
			},
			want: []string{`return {["foo"]="bar",}`},
		},
		{
			name: "number key and value",
			in: func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetInt(1, lua.LNumber(42))
				return tbl
			},
			want: []string{`return {[1]=42,}`},
		},
		{
			name: "boolean value",
			in: func(L *lua.LState) *lua.LTable {
				tbl := L.NewTable()
				tbl.RawSetString("flag", lua.LBool(true))
				return tbl
			},
			want: []string{`return {["flag"]=true,}`},
		},
		{
			name: "nested table",
			in: func(L *lua.LState) *lua.LTable {
				nested := L.NewTable()
				nested.RawSetString("a", lua.LNumber(1))
				nested.RawSetString("b", lua.LNumber(2))
				tbl := L.NewTable()
				tbl.RawSetString("nested", nested)
				return tbl
			},
			want: []string{
				`return {["nested"]={["a"]=1,["b"]=2,},}`,
				`return {["nested"]={["b"]=2,["a"]=1,},}`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			w := NewWriter(buf)

			L := lua.NewState()
			defer L.Close()
			tbl := tc.in(L)

			err := w.Write(tbl)
			if err != nil {
				t.Fatalf("Write() error: %v", err)
			}

			r := flate.NewReader(bytes.NewReader(buf.Bytes()))
			defer r.Close()
			data, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("ReadAll error: %v", err)
			}
			got := string(data)
			found := slices.Contains(tc.want, got)
			if !found {
				t.Errorf("got %q; want one of %q", got, tc.want)
			}
		})
	}
}
