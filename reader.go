/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

// Package jkr reads and writes Balatro save files (jkr).
// It will always be compatible with the latest Balatro version.
//
// A jkr file contains a Lua table as a string which is deflated with level 1.
package jkr

import (
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// A Reader reads the Lua table from a jkr file.
//
// As returned by NewReader, a Reader expects input that was generated from the
// official Balatro program.
type Reader struct {
	ir io.Reader
}

// NewReader returns a new Reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{ir: r}
}

// Read reads the Lua table from r. A successful call returns err == nil, not
// err == io.EOF. Because Read is defined to read until EOF, it does not treat end
// of file as an error to be reported.
func (r *Reader) Read() (*lua.LTable, error) {
	zr := flate.NewReader(r.ir)
	defer zr.Close()

	content, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}

	l := lua.NewState()
	defer l.Close()
	if err := l.DoString(fmt.Sprintf("zw_data = (%s)", strings.TrimPrefix(string(content), "return "))); err != nil {
		return nil, err
	}

	zwData, ok := l.GetGlobal("zw_data").(*lua.LTable)
	if !ok {
		return nil, errors.New("unable to typecast as lua.LTable")
	}

	return zwData, err
}
