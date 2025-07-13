/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package jkr

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func Marshal(in *lua.LTable) (out []byte, err error) {
	buf := &bytes.Buffer{}
	err = MarshalWrite(buf, in)
	return buf.Bytes(), err
}

func MarshalWrite(out io.Writer, in *lua.LTable) (err error) {
	zw, _ := flate.NewWriter(out, flate.BestSpeed)
	defer zw.Close()

	visited := make(map[*lua.LTable]bool)
	data, err := stringPack(in, false, visited)
	if err != nil {
		return err
	}

	if _, err := zw.Write([]byte(data)); err != nil {
		return err
	}

	return zw.Flush()
}

// stringPack serializes a lua.LTable into a Lua table literal string with cycle detection
func stringPack(data *lua.LTable, recursive bool, visited map[*lua.LTable]bool) (string, error) {
	// Check for cycles
	if visited[data] {
		return "", fmt.Errorf("circular reference detected in table")
	}
	visited[data] = true
	defer func() {
		delete(visited, data)
	}()

	var b strings.Builder
	if !recursive {
		b.WriteString("return ")
	}
	b.WriteString("{")

	var gerr error
	data.ForEach(func(key, value lua.LValue) {
		// serialize key
		var k string
		switch key.Type() {
		case lua.LTString:
			k = fmt.Sprintf("[%q]", key.String())
		case lua.LTNumber:
			k = fmt.Sprintf("[%v]", key)
		default:
			gerr = fmt.Errorf("invalid key type: table keys must be strings or numbers")
			return
		}
		// serialize value
		var v string
		var err error
		switch value.Type() {
		case lua.LTTable:
			tbl := value.(*lua.LTable)
			// detect Object tables by presence of an 'is' method without VM invocation
			fn := tbl.RawGetString("is")
			if fn.Type() == lua.LTFunction {
				v = "\"MANUAL_REPLACE\""
			} else {
				v, err = stringPack(tbl, true, visited)
				if err != nil {
					gerr = fmt.Errorf("error packing table value for key %s: %w", k, err)
					return
				}
			}
		case lua.LTString:
			v = fmt.Sprintf("%q", value.String())
		case lua.LTBool:
			if lua.LVAsBool(value) {
				v = "true"
			} else {
				v = "false"
			}
		case lua.LTNumber:
			v = fmt.Sprintf("%v", value)
		default:
			gerr = fmt.Errorf("unsupported value type %T for key %s", value, k)
			return
		}
		// serialize key-value pair
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString(",")
	})
	if gerr != nil {
		return "", gerr
	}
	b.WriteString("}")
	return b.String(), nil
}
