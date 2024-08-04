// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package riscv64asm

import (
	"bufio"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testDecode(t *testing.T, syntax string) {
	input := filepath.Join("testdata", syntax+"cases.txt")
	f, err := os.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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
		asm0 := strings.Replace(f[1], "	", " ", -1)
		asm := strings.TrimSpace(asm0)
		inst, decodeErr := Decode(code)
		if decodeErr != nil && decodeErr != errUnknown {
			if asm == "illegalins" && decodeErr == errShort {
				continue
			}
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
			out = GoSyntax(inst, 0, nil, nil)
		default:
			t.Errorf("unknown syntax %q", syntax)
			continue
		}

		if asm != out {
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
