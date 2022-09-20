// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ppc64asm

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestObjdumpPowerTestdata(t *testing.T) { testObjdump(t, testdataCases(t)) }
func TestObjdumpPowerManual(t *testing.T)   { testObjdump(t, hexCases(t, objdumpManualTests)) }

// Disable this for now since generating all possible bit combinations within a word
// generates lots of ppc64x instructions not possible with golang so not worth supporting..
//func TestObjdumpPowerRandom(t *testing.T)   { testObjdump(t, randomCases(t)) }

// objdumpManualTests holds test cases that will be run by TestObjdumpPowerManual.
// If you are debugging a few cases that turned up in a longer run, it can be useful
// to list them here and then use -run=Manual, particularly with tracing enabled.
// Note that these are byte sequences, so they must be reversed from the usual
// word presentation.
var objdumpManualTests = `
6d746162
4c040000
88000017
`

// allowedMismatchObjdump reports whether the mismatch between text and dec
// should be allowed by the test.
func allowedMismatchObjdump(text string, size int, inst *Inst, dec ExtInst) bool {
	// we support more instructions than binutils
	if strings.Contains(dec.text, ".long") {
		return true
	}

	switch inst.Op {
	case BC: // We don't print PC relative branches the same way.
		return true
	case DCBF, DCBT: // We only support extended mnemonics, and may not print 0 where R0 == 0.
		return true
	case MTVSRWA, MTVSRWZ, MFVSRWZ, MFVSRD, MTVSRD: // We don't support extended mnemonics using VRs or FPRs
		return true
	case ISEL: // We decode the BI similar to conditional branch insn, objdump doesn't.
		return true
	case SYNC, WAIT, RFEBB: // ISA 3.1 adds more bits and extended mnemonics for these book ii instructions.
		return true
	case BL:
		// TODO: Ignore these for now. The output format from gnu objdump is dependent on more than the
		// instruction itself e.g: decode(48100009) = "bl 0x100008", 4, want "bl .+0x100008", 4
		return true
	}

	if len(dec.enc) >= 4 {
		_ = binary.BigEndian.Uint32(dec.enc[:4])
	}

	return false
}
