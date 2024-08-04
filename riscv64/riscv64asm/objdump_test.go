// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package riscv64asm

import (
	"strings"
	"testing"
)

func TestObjdumpRISCV64TestDecodeGNUSyntaxdata(t *testing.T) {
	testObjdumpRISCV64(t, testdataCases(t, "gnu"))
}
func TestObjdumpRISCV64TestDecodeGoSyntaxdata(t *testing.T) {
	testObjdumpRISCV64(t, testdataCases(t, "plan9"))
}

func TestObjdumpRISCV64Manual(t *testing.T) {
	testObjdumpRISCV64(t, hexCases(t, objdumpManualTests))
}

// objdumpManualTests holds test cases that will be run by TestObjdumpRISCV64Manual.
// If you are debugging a few cases that turned up in a longer run, it can be useful
// to list them here and then use -run=Manual, particularly with tracing enabled.
// Note that these are byte sequences, so they must be reversed from the usual
// word presentation.
var objdumpManualTests = `
93020300
13000000
9b020300
afb5b50e
73b012c0
73f01fc0
73a012c0
73e01fc0
f3223000
f3221000
f3222000
f3123300
f3121300
f3122300
739012c0
73d01fc0
53a01022
53a01020
53801022
53801020
53901022
53901020
67800000
67800200
b3026040
bb026040
9342f3ff
f32200c0
f32200c8
f32220c0
f32220c8
f32210c0
f32210c8
`

// allowedMismatchObjdump reports whether the mismatch between text and dec
// should be allowed by the test.
func allowedMismatchObjdump(text string, inst *Inst, dec ExtInst) bool {
	// Allow the mismatch of Branch/Jump instruction's offset.
	decsp := strings.Split(dec.text, ",")

	switch inst.Op {
	case BEQ, BGE, BGEU, BLT, BLTU, BNE:
		if inst.Args[2].(Simm).String() != decsp[len(decsp)-1] {
			return true
		}
	case JAL:
		if inst.Args[1].(Simm).String() != decsp[len(decsp)-1] {
			return true
		}
	case JALR:
		if inst.Args[1].(RegOffset).Ofs.String() != decsp[len(decsp)-1] {
			return true
		}
	}

	return false
}
