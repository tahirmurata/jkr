/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package jkr

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func Unmarshal(in []byte, out *lua.LTable) (err error) {
	br := bytes.NewReader(in)
	return UnmarshalRead(br, out)
}

func UnmarshalRead(in io.Reader, out *lua.LTable) (err error) {
	zr := flate.NewReader(in)
	defer zr.Close()

	content, err := io.ReadAll(zr)
	if err != nil {
		return err
	}

	l := lua.NewState()
	defer l.Close()
	if err := l.DoString(fmt.Sprintf("zw_data = (%s)", strings.TrimPrefix(string(content), "return "))); err != nil {
		return err
	}

	zwData, ok := l.GetGlobal("zw_data").(*lua.LTable)
	if !ok {
		return errors.New("unable to typecast as lua.LTable")
	}

	*out = *zwData

	return err
}
