/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package jkr

import (
	"compress/flate"
	"fmt"
	"io"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// stringPack serializes a lua.LTable into a Lua table literal string
func stringPack(data *lua.LTable, recursive bool) (string, error) {
	var b strings.Builder
	if !recursive {
		b.WriteString("return ")
	}
	b.WriteString("{")

	var firstError error
	data.ForEach(func(key, value lua.LValue) {
		if firstError != nil {
			return
		}

		// serialize key
		var k string
		switch key.Type() {
		case lua.LTString:
			k = fmt.Sprintf("[%q]", key.String())
		case lua.LTNumber:
			k = fmt.Sprintf("[%v]", key)
		default:
			firstError = fmt.Errorf("invalid key type: table keys must be strings or numbers")
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
				v, err = stringPack(tbl, true)
				if err != nil {
					firstError = fmt.Errorf("error packing table value for key %s: %w", k, err)
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
			firstError = fmt.Errorf("unsupported value type %T for key %s", value, k)
			return
		}
		// serialize key-value pair
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString(",")
	})
	if firstError != nil {
		return "", firstError
	}
	b.WriteString("}")
	return b.String(), nil
}

// A Writer writes the Lua table in a jkr format.
//
// As returned by NewWriter, a Writer writes the Lua table such that it is
// compatible with the official Balatro program.
type Writer struct {
	iw io.Writer
}

// NewWriter returns a new Writer that writes to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{iw: w}
}

// Write writes the Lua table to w and then calls Flush on the flate writer,
// returning any error from the Flush.
func (w *Writer) Write(in *lua.LTable) error {
	zw, err := flate.NewWriter(w.iw, flate.BestSpeed)
	if err != nil {
		return err
	}
	defer zw.Close()

	data, err := stringPack(in, false)
	if err != nil {
		return err
	}

	if _, err := zw.Write([]byte(data)); err != nil {
		return err
	}

	return zw.Flush()
}
