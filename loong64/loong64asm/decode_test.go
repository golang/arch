// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loong64asm

import (
	"encoding/hex"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func testDecode(t *testing.T, syntax string) {
	input := filepath.Join("testdata", syntax+"cases.txt")
	data, err := ioutil.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	all := string(data)
	for strings.Contains(all, "\t\t") {
		all = strings.Replace(all, "\t\t", "\t", -1)
	}
	for _, line := range strings.Split(all, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, "\t", 2)
		i := strings.Index(f[0], "|")
		if i < 0 {
			t.Errorf("parsing %q: missing | separator", f[0])
			continue
		}
		if i%2 != 0 {
			t.Errorf("parsing %q: misaligned | separator", f[0])
		}
		code, err := hex.DecodeString(f[0][:i] + f[0][i+1:])
		if err != nil {
			t.Errorf("parsing %q: %v", f[0], err)
			continue
		}
		asm := f[1]
		inst, decodeErr := Decode(code)
		if decodeErr != nil && decodeErr != errUnknown {
			// Some rarely used system instructions are not supported
			// Following logicals will filter such unknown instructions
			t.Errorf("parsing %x: %s", code, decodeErr)
			continue
		}
		var out string
		switch syntax {
		case "gnu":
			out = GNUSyntax(inst)
		case "plan9":
			out = GoSyntax(inst, 0, nil)
		default:
			t.Errorf("unknown syntax %q", syntax)
			continue
		}

		// var out string
		if asm != out || len(asm) != len(out) {
			t.Errorf("Decode(%s) [%s] = %s want %s", f[0], syntax, out, asm)
		}
	}
}

func TestDecodeGNUSyntax(t *testing.T) {
	testDecode(t, "gnu")
}

func TestDecodeGoSyntax(t *testing.T) {
	testDecode(t, "plan9")
}
