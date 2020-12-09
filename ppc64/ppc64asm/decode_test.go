// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ppc64asm

import (
	"encoding/binary"
	"encoding/hex"
	"io/ioutil"
	"path"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	files, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), "decode") {
			continue
		}
		filename := path.Join("testdata", f.Name())
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		decode(data, t, filename)
	}
}

func decode(data []byte, t *testing.T, filename string) {
	all := string(data)
	// Simulate PC based on number of instructions found in the test file.
	pc := uint64(0)
	for strings.Contains(all, "\t\t") {
		all = strings.Replace(all, "\t\t", "\t", -1)
	}
	for _, line := range strings.Split(all, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, "\t", 3)
		i := strings.Index(f[0], "|")
		if i < 0 {
			t.Errorf("%s: parsing %q: missing | separator", filename, f[0])
			continue
		}
		if i%2 != 0 {
			t.Errorf("%s: parsing %q: misaligned | separator", filename, f[0])
		}
		size := i / 2
		code, err := hex.DecodeString(f[0][:i] + f[0][i+1:])
		if err != nil {
			t.Errorf("%s: parsing %q: %v", filename, f[0], err)
			continue
		}
		syntax, asm := f[1], f[2]
		inst, err := Decode(code, binary.BigEndian)
		var out string
		if err != nil {
			out = "error: " + err.Error()
		} else {
			switch syntax {
			case "gnu":
				out = GNUSyntax(inst, pc)
			case "plan9":
				out = GoSyntax(inst, pc, nil)
			default:
				t.Errorf("unknown syntax %q", syntax)
				continue
			}
		}
		pc += uint64(size)
		if out != asm || inst.Len != size {
			t.Errorf("%s: Decode(%s) [%s] = %s want %s", filename, f[0], syntax, out, asm)
		}
	}
}
