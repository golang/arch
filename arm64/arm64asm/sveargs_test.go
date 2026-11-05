// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

import (
	"testing"
)

func TestSVERegisterFiles(t *testing.T) {
	// The new files must sit above every existing register and not overlap.
	if Z0 != V31+1 {
		t.Errorf("Z0 = %d, want %d", Z0, V31+1)
	}
	if P0 != Z31+1 || PN0 != P15+1 {
		t.Errorf("register files are not contiguous: Z31=%d P0=%d P15=%d PN0=%d", Z31, P0, P15, PN0)
	}
	for _, tt := range []struct {
		r    Reg
		want string
	}{
		{Z0, "Z0"}, {Z31, "Z31"}, {P0, "P0"}, {P15, "P15"}, {PN0, "PN0"}, {PN15, "PN15"},
		// The existing files are untouched.
		{V0, "V0"}, {V31, "V31"}, {X30, "X30"}, {XZR, "XZR"}, {WZR, "WZR"}, {Q31, "Q31"},
		// A value that is in no file still falls through to the old spelling.
		{PN15 + 1, "Reg(320)"},
	} {
		if got := tt.r.String(); got != tt.want {
			t.Errorf("Reg(%d).String() = %q, want %q", int(tt.r), got, tt.want)
		}
	}
	for r := Z0; r <= PN15; r++ {
		base, size := fileBase(r), fileSize(r)
		if r < base || r >= base+Reg(size) {
			t.Errorf("%v is outside its own file [%v, +%d)", r, base, size)
		}
	}
	for _, r := range []Reg{W0, X0, XZR, B0, Q31, V0, V31} {
		if fileBase(r) != V0 || fileSize(r) != 32 {
			t.Errorf("%v: fileBase = %v, fileSize = %d, want V0, 32", r, fileBase(r), fileSize(r))
		}
	}
}

func TestSVEArrangements(t *testing.T) {
	for _, tt := range []struct {
		a    Arrangement
		want string
	}{
		{ArrangementM, "/M"}, {ArrangementZ, "/Z"}, {ArrangementQ, ".Q"},
		{ArrangementB, ".B"}, {Arrangement16B, ".16B"}, {Arrangement1Q, ".1Q"}, {Arrangement2D, ".2D"},
		{0, ""},
	} {
		if got := tt.a.String(); got != tt.want {
			t.Errorf("Arrangement(%d).String() = %q, want %q", tt.a, got, tt.want)
		}
	}
	if (RegisterWithArrangement{r: P4, a: ArrangementM}).String() != "P4/M" {
		t.Errorf("predicate with merging qualifier = %q", RegisterWithArrangement{r: P4, a: ArrangementM})
	}
}

func TestSVEListSpelling(t *testing.T) {
	const rng = sveRangeBit
	for _, tt := range []struct {
		arg  Arg
		want string
	}{
		// A bare register: cnt == 0 means "not a list" for every file.
		{RegisterWithArrangement{r: Z3, a: ArrangementS}, "Z3.S"},
		{RegisterWithArrangement{r: P7}, "P7"},
		{RegisterWithArrangement{r: PN8}, "PN8"},
		// Lists: ARM enumerates a pair and ranges from three up.
		{RegisterWithArrangement{r: Z0, a: ArrangementB, cnt: 1}, "{ Z0.B }"},
		{RegisterWithArrangement{r: Z13, a: ArrangementS, cnt: 2}, "{ Z13.S, Z14.S }"},
		{RegisterWithArrangement{r: Z4, a: ArrangementD, cnt: 3}, "{ Z4.D - Z6.D }"},
		{RegisterWithArrangement{r: Z28, a: ArrangementH, cnt: 4}, "{ Z28.H - Z31.H }"},
		// A group that wraps is enumerated, within the Z file and not the V file.
		{RegisterWithArrangement{r: Z30, a: ArrangementB, cnt: 4}, "{ Z30.B, Z31.B, Z0.B, Z1.B }"},
		{RegisterWithArrangement{r: Z31, a: ArrangementB, cnt: 2}, "{ Z31.B, Z0.B }"},
		// The P files have 16 registers, so they wrap at 16.
		{RegisterWithArrangement{r: P14, cnt: 4}, "{ P14, P15, P0, P1 }"},
		{RegisterWithArrangement{r: PN12, cnt: 4}, "{ PN12 - PN15 }"},
		{RegisterWithArrangement{r: P15, cnt: 2}, "{ P15, P0 }"},
		{RegisterWithArrangement{r: PN15, cnt: 2}, "{ PN15, PN0 }"},
		// The range marker is Go-syntax information and does not change GNU syntax.
		{RegisterWithArrangement{r: Z22, a: ArrangementB, cnt: 2 | rng}, "{ Z22.B, Z23.B }"},
		{RegisterWithArrangement{r: Z4, a: ArrangementD, cnt: 3 | rng}, "{ Z4.D - Z6.D }"},
		{RegisterWithArrangement{r: Z0, a: ArrangementS, cnt: rng}, "Z0.S"},
		// Indexed forms.
		{RegisterWithArrangementAndIndex{r: Z5, a: ArrangementS, index: 3}, "Z5.S[3]"},
		{RegisterWithArrangementAndIndex{r: Z5, a: ArrangementQ, index: 1, cnt: 2}, "{ Z5.Q, Z6.Q }[1]"},
		{RegisterWithArrangementAndIndex{r: Z5, a: ArrangementD, index: 1, cnt: 4 | rng}, "{ Z5.D - Z8.D }[1]"},
	} {
		if got := tt.arg.String(); got != tt.want {
			t.Errorf("%#v = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

// TestNEONListSpellingUnchanged pins the NEON spellings that share code with
// the SVE list printer.
func TestNEONListSpellingUnchanged(t *testing.T) {
	for _, tt := range []struct {
		arg  Arg
		want string
	}{
		{RegisterWithArrangement{V5, Arrangement4S, 0}, "V5.4S"},
		{RegisterWithArrangement{V5, Arrangement8B, 1}, "{V5.8B}"},
		{RegisterWithArrangement{V5, Arrangement8B, 2}, "{V5.8B, V6.8B}"},
		{RegisterWithArrangement{V5, Arrangement16B, 3}, "{V5.16B-V7.16B}"},
		{RegisterWithArrangement{V30, Arrangement16B, 4}, "{V30.16B, V31.16B, V0.16B, V1.16B}"},
		{RegisterWithArrangementAndIndex{V2, ArrangementB, 7, 0}, "V2.B[7]"},
		{RegisterWithArrangementAndIndex{V2, ArrangementB, 7, 1}, "{V2.B}[7]"},
		{RegisterWithArrangementAndIndex{V2, ArrangementD, 1, 2}, "{V2.D, V3.D}[1]"},
	} {
		if got := tt.arg.String(); got != tt.want {
			t.Errorf("%#v = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

func TestSVEArgs(t *testing.T) {
	for _, tt := range []struct {
		arg  Arg
		want string
	}{
		{ImmSigned(0), "#0"},
		{ImmSigned(-2), "#-2"},
		{ImmSigned(-1 << 63), "#-9223372036854775808"},
		{ImmSigned(31), "#31"},

		// The zero offset is spelled for MUL VL.
		{MemScaledVL{Base: RegSP(X7)}, "[X7, #0, MUL VL]"},
		{MemScaledVL{Base: RegSP(X7), Imm: -8}, "[X7, #-8, MUL VL]"},
		{MemScaledVL{Base: RegSP(XZR), Imm: 7}, "[SP, #7, MUL VL]"},

		{MemVectorBase{Base: Z3, Arr: ArrangementS}, "[Z3.S]"},
		{MemVectorBase{Base: Z3, Arr: ArrangementD, Imm: 248}, "[Z3.D, #248]"},

		{MemVectorIndex{Base: X1, Index: Z2, IdxArr: ArrangementS, Extend: uxtw}, "[X1, Z2.S, UXTW]"},
		{MemVectorIndex{Base: X1, Index: Z2, IdxArr: ArrangementD, Extend: lsl, Amount: 3}, "[X1, Z2.D, LSL #3]"},
		{MemVectorIndex{Base: X1, Index: Z2, IdxArr: ArrangementS, Extend: sxtw, Amount: 2}, "[X1, Z2.S, SXTW #2]"},
		{MemVectorIndex{Base: X1, Index: Z2, IdxArr: ArrangementD}, "[X1, Z2.D]"},
		// Register 31 as a general purpose base is SP, never XZR.
		{MemVectorIndex{Base: XZR, Index: Z2, IdxArr: ArrangementD}, "[SP, Z2.D]"},
		// A vector base (ADR) carries its own arrangement.
		{MemVectorIndex{Base: Z1, BaseArr: ArrangementS, Index: Z2, IdxArr: ArrangementS, Extend: lsl, Amount: 2}, "[Z1.S, Z2.S, LSL #2]"},
		{MemVectorIndex{Base: Z1, BaseArr: ArrangementD, Index: Z2, IdxArr: ArrangementD, Extend: sxtw}, "[Z1.D, Z2.D, SXTW]"},

		{VLx2, "vlx2"},
		{VLx4, "vlx4"},
	} {
		if got := tt.arg.String(); got != tt.want {
			t.Errorf("%#v = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

// TestSVEGNUSyntax runs the new operands through GNUSyntax, which lower-cases
// what their String methods return. Two of these spellings are invisible in Go
// syntax, where register 31 is RSP and a SIMD&FP register is V5 whatever its
// arrangement.
func TestSVEGNUSyntax(t *testing.T) {
	for _, tt := range []struct {
		name string
		args Args
		want string
	}{
		{"base is sp not xzr", Args{Z0, P1, MemVectorIndex{Base: XZR, Index: Z2, IdxArr: ArrangementD}}, "add z0, p1, [sp, z2.d]"},
		{"arranged SIMD&FP register is v5.4s", Args{RegisterWithArrangement{V5, Arrangement4S, 0}, Z1}, "add v5.4s, z1"},
		{"predicate qualifier", Args{Z0, RegisterWithArrangement{r: P4, a: ArrangementM}, Z1}, "add z0, p4/m, z1"},
		{"signed immediate", Args{Z0, ImmSigned(-16)}, "add z0, #-16"},
		{"mul vl", Args{Z0, P1, MemScaledVL{Base: RegSP(X3), Imm: 4}}, "add z0, p1, [x3, #4, mul vl]"},
		{"vector base", Args{Z0, P1, MemVectorBase{Base: Z3, Arr: ArrangementS, Imm: 4}}, "add z0, p1, [z3.s, #4]"},
		{"list", Args{RegisterWithArrangement{r: Z6, a: ArrangementH, cnt: 3 | sveRangeBit}, VLx4}, "add { z6.h - z8.h }, vlx4"},
		{"pn", Args{PN8, PN15}, "add pn8, pn15"},
	} {
		got := GNUSyntax(Inst{Op: ADD, Args: tt.args})
		if got != tt.want {
			t.Errorf("%s: GNUSyntax = %q, want %q", tt.name, got, tt.want)
		}
	}
}
