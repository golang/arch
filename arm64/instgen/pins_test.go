// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWritePins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pins.txt")
	writePins(path, []e2eData{
		{GoOp: "PAND", Asm: "PAND P4.B, P2.B, P1.Z, P14.B                      ", Binary: "// 4e440425"},
		{GoOp: "CMPLE", Asm: "// TODO: CMPLE"},
		{GoOp: "ZABS", Asm: "ZABS Z1.B, P2.M, Z3.B   ", Binary: "// 63a8040a"},
	})
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := pinsHeader + "4e440425|\tPAND P4.B, P2.B, P1.Z, P14.B\n63a8040a|\tZABS Z1.B, P2.M, Z3.B\n"
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}
