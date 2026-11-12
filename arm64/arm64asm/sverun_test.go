// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"math/rand"
	"strings"
	"testing"
)

// sveTables is a complete set of the tables that sverun.go reads.
type sveTables struct {
	opARM    []string
	opGo     []string
	syms     []string
	descs    []sveDesc
	args     []sveArg
	argLists [][]uint16
	forms    []sveForm
}

// useSVETables installs tab for the rest of the test. The tables are package
// variables, so a test that calls this must not run in parallel.
func useSVETables(t *testing.T, tab sveTables) {
	t.Helper()
	old := sveTables{sveOpARM, sveOpGo, sveSyms, sveDescs, sveArgs, sveArgLists, sveForms}
	oldLo, oldHi := sveGateLo, sveGateHi
	set := func(x sveTables) {
		sveOpARM, sveOpGo, sveSyms, sveDescs, sveArgs, sveArgLists, sveForms =
			x.opARM, x.opGo, x.syms, x.descs, x.args, x.argLists, x.forms
	}
	set(tab)
	// The gate belongs to the forms it was generated from, so a table set made by
	// hand gets the gate its own forms need.
	sveGateLo, sveGateHi = sveGateOf(tab.forms)
	t.Cleanup(func() { set(old); sveGateLo, sveGateHi = oldLo, oldHi })
}

// sveGateOf computes the bitmap of values of bits 31:25 that any of forms can
// match. It is the reference for the generated sveGateLo and sveGateHi.
func sveGateOf(forms []sveForm) (lo, hi uint64) {
	for v := uint32(0); v < 128; v++ {
		for _, f := range forms {
			if v&(f.mask>>25) == f.value>>25&(f.mask>>25) {
				if v < 64 {
					lo |= 1 << v
				} else {
					hi |= 1 << (v - 64)
				}
				break
			}
		}
	}
	return lo, hi
}

// sveTestTables is a hand-maintained table set for 34 SVE forms, chosen so that
// every operand kind, every register file and every knob of sveDesc is used by
// at least one of them (TestSVETestTablesCoverRuntime checks that). Each form's
// comment is the ARM manual's syntax for it. The indexes in the comments are the
// indexes the rows refer to.
var sveTestTables = sveTables{
	opARM: []string{"ADD", "ADDQV", "ADR", "ASR", "CNTP", "CPY", "CTERMEQ", "CTERMEQ", "DUP", "DUPM", "FADD", "FADDV", "FCADD", "FCMEQ", "FCMLA", "FCPY", "INDEX", "LD1B", "LD1RB", "LD1W", "LDNT1W", "LSL", "PEXT", "PRFB", "PRFD", "SADDV", "SQRSHRN", "ST1Q", "ST2B", "UQDECP", "WHILELT"},
	opGo:  []string{"ZADD", "ZADDQV", "ZADR", "ZASR", "PCNTP", "ZCPY", "CTERMEQW", "CTERMEQ", "ZDUP", "ZDUPM", "ZFADD", "ZFADDVD", "ZFCADD", "ZFCMEQ", "ZFCMLA", "ZFCPY", "ZINDEX", "ZLD1B", "ZLD1RB", "ZLD1W", "ZLDNT1W", "ZLSL", "PPEXT", "ZPRFB", "PPRFD", "ZSADDVD", "ZSQRSHRN", "ZST1Q", "ZST2B", "PUQDECPW", "PWHILELT"},
	syms:  []string{"B", "H", "S", "D", "M", "Z", "16B", "8H", "4S", "2D", "[absent]", "LSL", "#1", "#2", "#3", "SXTW", "UXTW", "Q", "#90", "#270", "#0", "#180", "VLx2", "VLx4", "#0.5", "#1.0", "#0.0", "PLDL1KEEP", "PLDL1STRM", "PLDL2KEEP", "PLDL2STRM", "PLDL3KEEP", "PLDL3STRM", "#uimm4", "PSTL1KEEP", "PSTL1STRM", "PSTL2KEEP", "PSTL2STRM", "PSTL3KEEP", "PSTL3STRM"},
	descs: []sveDesc{
		/*  0*/ {role: roleReg, fields: []sveField{{5, 5}}, regfile: rfZ, fixed: -1},
		/*  1*/ {role: roleArng, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 0}, {0x3, 0x1, 1}, {0x3, 0x2, 2}, {0x3, 0x3, 3}}},
		/*  2*/ {role: roleReg, fields: []sveField{{10, 3}}, hasRange: true, min: 0, max: 7, regfile: rfP, fixed: -1},
		/*  3*/ {role: roleQual, fixed: 4},
		/*  4*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfZ, fixed: -1},
		/*  5*/ {role: roleQual, fixed: 5},
		/*  6*/ {role: roleReg, fields: []sveField{{16, 5}}, regfile: rfZ, fixed: -1},
		/*  7*/ {role: roleArng, fields: []sveField{{22, 1}}, fixed: -1, table: []sveRow{{0x1, 0x0, 2}, {0x1, 0x1, 3}}},
		/*  8*/ {role: roleArng, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x1, 1}, {0x3, 0x2, 2}, {0x3, 0x3, 3}}},
		/*  9*/ {role: roleArng, fixed: 3},
		/* 10*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfV, fixed: -1},
		/* 11*/ {role: roleArng, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 6}, {0x3, 0x1, 7}, {0x3, 0x2, 8}, {0x3, 0x3, 9}}},
		/* 12*/ {role: roleMod, fields: []sveField{{10, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 10}, {0x1, 0x1, 11}, {0x3, 0x2, 11}}},
		/* 13*/ {role: roleAmount, fields: []sveField{{10, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 10}, {0x3, 0x1, 12}, {0x3, 0x2, 13}, {0x3, 0x3, 14}}},
		/* 14*/ {role: roleMod, fixed: 15},
		/* 15*/ {role: roleArng, fixed: 17},
		/* 16*/ {role: roleArng, fixed: 0},
		/* 17*/ {role: roleImm, fields: []sveField{{5, 13}}, bitmask: true, fixed: -1},
		/* 18*/ {role: roleArng, fields: []sveField{{5, 13}}, fixed: -1, table: []sveRow{{0x1020, 0x0, 2}, {0x1030, 0x20, 1}, {0x1038, 0x30, 0}, {0x103c, 0x38, 0}, {0x103e, 0x3c, 0}, {0x1000, 0x1000, 3}}},
		/* 19*/ {role: roleReg, fields: []sveField{{16, 4}}, regfile: rfP, fixed: -1},
		/* 20*/ {role: roleReg, fields: []sveField{{5, 4}}, regfile: rfP, fixed: -1},
		/* 21*/ {role: roleReg, fields: []sveField{{0, 4}}, regfile: rfP, fixed: -1},
		/* 22*/ {role: roleImm, fields: []sveField{{16, 3}, {19, 2}, {22, 2}}, tsz: tszShiftR, fixed: -1},
		/* 23*/ {role: roleArng, fields: []sveField{{19, 2}, {22, 2}}, fixed: -1, table: []sveRow{{0xf, 0x1, 0}, {0xe, 0x2, 1}, {0xc, 0x4, 2}, {0x8, 0x8, 3}}},
		/* 24*/ {role: roleArng, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 0}, {0x3, 0x1, 1}, {0x3, 0x2, 2}}},
		/* 25*/ {role: roleArng, fixed: 1},
		/* 26*/ {role: roleArng, fixed: 2},
		/* 27*/ {role: roleReg, fields: []sveField{{6, 4}}, stride: 2, regfile: rfZ, fixed: -1},
		/* 28*/ {role: roleReg, fields: []sveField{{6, 4}}, stride: 2, bias: 1, regfile: rfZ, fixed: -1},
		/* 29*/ {role: roleImm, fields: []sveField{{10, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 20}, {0x3, 0x1, 18}, {0x3, 0x2, 21}, {0x3, 0x3, 19}}},
		/* 30*/ {role: roleReg, fields: []sveField{{16, 4}}, hasRange: true, min: 0, max: 15, regfile: rfZ, fixed: -1},
		/* 31*/ {role: roleIndex, fields: []sveField{{20, 1}}, hasRange: true, min: 0, max: 1, fixed: -1},
		/* 32*/ {role: roleVL, fields: []sveField{{10, 1}}, fixed: -1, table: []sveRow{{0x1, 0x0, 22}, {0x1, 0x1, 23}}},
		/* 33*/ {role: roleReg, fields: []sveField{{5, 4}}, regfile: rfPN, fixed: -1},
		/* 34*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfR64, fixed: -1},
		/* 35*/ {role: roleImm, fields: []sveField{{5, 8}}, signed: true, hasRange: true, min: -128, max: 127, altBit: 13, altShift: 8, fixed: -1},
		/* 36*/ {role: roleReg, fields: []sveField{{16, 5}}, regfile: rfRauto, fixed: -1},
		/* 37*/ {role: roleReg, fields: []sveField{{5, 5}}, regfile: rfRauto, fixed: -1},
		/* 38*/ {role: roleArng, fields: []sveField{{16, 5}}, fixed: -1, table: []sveRow{{0x1, 0x1, 0}, {0x3, 0x2, 1}, {0x7, 0x4, 2}, {0xf, 0x8, 3}, {0x1f, 0x10, 17}}},
		/* 39*/ {role: roleIndex, fields: []sveField{{16, 5}, {22, 2}}, tsz: tszIndex, fixed: -1},
		/* 40*/ {role: roleImm, fields: []sveField{{5, 1}}, fixed: -1, table: []sveRow{{0x1, 0x0, 24}, {0x1, 0x1, 25}}},
		/* 41*/ {role: roleImm, fields: []sveField{{16, 1}}, fixed: -1, table: []sveRow{{0x1, 0x0, 18}, {0x1, 0x1, 19}}},
		/* 42*/ {role: roleImm, fixed: 26},
		/* 43*/ {role: roleImm, fields: []sveField{{5, 8}}, fp8: true, fixed: -1},
		/* 44*/ {role: roleReg, fields: []sveField{{5, 5}}, regfile: rfR64, sp: true, fixed: -1},
		/* 45*/ {role: roleOff, fields: []sveField{{16, 4}}, signed: true, hasRange: true, min: -16, max: 14, stride: 2, fixed: -1},
		/* 46*/ {role: roleReg, fields: []sveField{{16, 5}}, regfile: rfR64, fixed: -1},
		/* 47*/ {role: roleMod, fields: []sveField{{22, 1}}, fixed: -1, table: []sveRow{{0x1, 0x0, 16}, {0x1, 0x1, 15}}},
		/* 48*/ {role: roleOff, fields: []sveField{{16, 5}}, hasRange: true, min: 0, max: 31, fixed: -1},
		/* 49*/ {role: roleMod, fixed: 11},
		/* 50*/ {role: roleAmount, fixed: 14},
		/* 51*/ {role: roleReg, fields: []sveField{{16, 5}}, regfile: rfR64, fixed: -1},
		/* 52*/ {role: roleOff, fields: []sveField{{16, 6}}, hasRange: true, min: 0, max: 63, fixed: -1},
		/* 53*/ {role: roleAmount, fixed: 13},
		/* 54*/ {role: roleOff, fields: []sveField{{16, 5}}, hasRange: true, min: 0, max: 124, stride: 4, fixed: -1},
		/* 55*/ {role: roleReg, fields: []sveField{{0, 5}}, bias: 1, modulo: 32, regfile: rfZ, fixed: -1},
		/* 56*/ {role: roleImm, fields: []sveField{{16, 3}, {19, 2}, {22, 2}}, tsz: tszShiftL, fixed: -1},
		/* 57*/ {role: roleReg, fields: []sveField{{5, 3}}, hasRange: true, min: 8, max: 15, bias: 8, regfile: rfPN, fixed: -1},
		/* 58*/ {role: roleIndex, fields: []sveField{{8, 2}}, hasRange: true, min: 0, max: 3, fixed: -1},
		/* 59*/ {role: rolePrfop, fields: []sveField{{0, 4}}, fixed: -1, table: []sveRow{{0xf, 0x0, 27}, {0xf, 0x1, 28}, {0xf, 0x2, 29}, {0xf, 0x3, 30}, {0xf, 0x4, 31}, {0xf, 0x5, 32}, {0x6, 0x6, 33}, {0xf, 0x8, 34}, {0xf, 0x9, 35}, {0xf, 0xa, 36}, {0xf, 0xb, 37}, {0xf, 0xc, 38}, {0xf, 0xd, 39}}},
		/* 60*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfV, vwidth: 64, fixed: -1},
		/* 61*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfR32, fixed: -1},
		/* 62*/ {role: roleImm, fields: []sveField{{16, 4}}, hasRange: true, min: 1, max: 16, invert: 16, fixed: -1},
	},
	args: []sveArg{
		/*  0*/ {kind: kArng, descs: []uint16{0, 1}}, // <Zn>.<T>
		/*  1*/ {kind: kPredQual, descs: []uint16{2, 3}}, // <Pg>/M
		/*  2*/ {kind: kArng, descs: []uint16{4, 1}}, // <Zd>.<T>
		/*  3*/ {kind: kPredQual, descs: []uint16{2, 5}}, // <Pg>/Z
		/*  4*/ {kind: kArng, descs: []uint16{4, 7}}, // <Zda>.<T>
		/*  5*/ {kind: kArng, descs: []uint16{0, 8}}, // <Zn>.<Tb>
		/*  6*/ {kind: kArng, descs: []uint16{4, 9}}, // <Zdn>.D
		/*  7*/ {kind: kReg, descs: []uint16{2}}, // <Pg>
		/*  8*/ {kind: kArng, descs: []uint16{10, 11}}, // <Vd>.<T>
		/*  9*/ {kind: kMemVecIndex, descs: []uint16{0, 7, 6, 7, 12, 13}}, // [<Zn>.<T>, <Zm>.<T>{, <mod> <amount>}]
		/* 10*/ {kind: kMemVecIndex, descs: []uint16{0, 9, 6, 9, 14, 13}}, // [<Zn>.D, <Zm>.D, SXTW{<amount>}]
		/* 11*/ {kind: kImm, descs: []uint16{17}}, // #<const>
		/* 12*/ {kind: kArng, descs: []uint16{4, 18}}, // <Zdn>.<T>
		/* 13*/ {kind: kReg, descs: []uint16{10}}, // <V><d>
		/* 14*/ {kind: kImm, descs: []uint16{22}}, // #<const>
		/* 15*/ {kind: kArng, descs: []uint16{0, 23}}, // <Zn>.<T>
		/* 16*/ {kind: kArng, descs: []uint16{4, 23}}, // <Zd>.<T>
		/* 17*/ {kind: kArng, descs: []uint16{0, 24}}, // <Zn>.<T>
		/* 18*/ {kind: kArng, descs: []uint16{4, 25}}, // <Zd>.H
		/* 19*/ {kind: kArng, descs: []uint16{0, 26}}, // <Zn>.S
		/* 20*/ {kind: kArng, descs: []uint16{4, 26}}, // <Zda>.S
		/* 21*/ {kind: kImm, descs: []uint16{29}}, // <const>
		/* 22*/ {kind: kArngIdx, descs: []uint16{30, 26, 31}}, // <Zm>.S[<imm>]
		/* 23*/ {kind: kArng, descs: []uint16{21, 1}}, // <Pd>.<T>
		/* 24*/ {kind: kSpecial, descs: []uint16{32}}, // <vl>
		/* 25*/ {kind: kArng, descs: []uint16{33, 1}}, // <PNn>.<T>
		/* 26*/ {kind: kReg, descs: []uint16{34}}, // <Xd>
		/* 27*/ {kind: kArng, descs: []uint16{20, 1}}, // <Pn>.<T>
		/* 28*/ {kind: kImm, descs: []uint16{35}}, // #<imm>{, <shift>}
		/* 29*/ {kind: kPredQual, descs: []uint16{19, 3}}, // <Pg>/M
		/* 30*/ {kind: kReg, descs: []uint16{36}}, // <R><m>
		/* 31*/ {kind: kReg, descs: []uint16{37}}, // <R><n>
		/* 32*/ {kind: kArng, descs: []uint16{4, 8}}, // <Zdn>.<T>
		/* 33*/ {kind: kArngIdx, descs: []uint16{0, 38, 39}}, // <Zn>.<T>[<imm>]
		/* 34*/ {kind: kArng, descs: []uint16{4, 38}}, // <Zd>.<T>
		/* 35*/ {kind: kArng, descs: []uint16{21, 8}}, // <Pd>.<T>
		/* 36*/ {kind: kImm, descs: []uint16{40}}, // <const>
		/* 37*/ {kind: kImm, descs: []uint16{41}}, // <const>
		/* 38*/ {kind: kImm, descs: []uint16{42}}, // #0.0
		/* 39*/ {kind: kImm, descs: []uint16{43}}, // #<const>
		/* 40*/ {kind: kListRange, descs: []uint16{27, 26, 28, 26}}, // { <Zn1>.S-<Zn2>.S }
		/* 41*/ {kind: kMemMulVL, descs: []uint16{44, 45}}, // [<Xn|SP>{, #<imm>, MUL VL}]
		/* 42*/ {kind: kList, descs: []uint16{4, 16}}, // { <Zt>.B }
		/* 43*/ {kind: kList, descs: []uint16{4, 26}}, // { <Zt>.S }
		/* 44*/ {kind: kMemExt, descs: []uint16{44, 46}}, // [<Xn|SP>, <Xm>]
		/* 45*/ {kind: kMemVecIndex, descs: []uint16{44, 6, 26, 47}}, // [<Xn|SP>, <Zm>.S, <mod>]
		/* 46*/ {kind: kMemVecBase, descs: []uint16{0, 26, 48}}, // [<Zn>.S{, #<imm>}]
		/* 47*/ {kind: kList, descs: []uint16{4, 15}}, // { <Zt>.Q }
		/* 48*/ {kind: kMemExt, descs: []uint16{44, 46, 49, 50}}, // [<Xn|SP>, <Xm>, LSL #3]
		/* 49*/ {kind: kMemVecIndex, descs: []uint16{0, 9, 51}}, // [<Zn>.D{, <Xm>}]
		/* 50*/ {kind: kMemOff, descs: []uint16{44, 52}}, // [<Xn|SP>{, #<imm>}]
		/* 51*/ {kind: kMemExt, descs: []uint16{44, 46, 49, 53}}, // [<Xn|SP>, <Xm>, LSL #2]
		/* 52*/ {kind: kMemVecBase, descs: []uint16{0, 26, 54}}, // [<Zn>.S{, #<imm>}]
		/* 53*/ {kind: kList, descs: []uint16{4, 16, 55, 16}}, // { <Zt1>.B, <Zt2>.B }
		/* 54*/ {kind: kImm, descs: []uint16{56}}, // #<const>
		/* 55*/ {kind: kRegIdx, descs: []uint16{57, 58}}, // <PNn>[<imm>]
		/* 56*/ {kind: kSpecial, descs: []uint16{59}}, // <prfop>
		/* 57*/ {kind: kReg, descs: []uint16{60}}, // <Dd>
		/* 58*/ {kind: kReg, descs: []uint16{61}}, // <Wdn>
		/* 59*/ {kind: kImm, descs: []uint16{62}}, // #<const>
	},
	argLists: [][]uint16{
		/*  0*/ {2, 1, 2, 0},
		/*  1*/ {8, 7, 0},
		/*  2*/ {4, 9},
		/*  3*/ {6, 10},
		/*  4*/ {16, 15, 14},
		/*  5*/ {20, 19, 22, 21},
		/*  6*/ {26, 25, 24},
		/*  7*/ {2, 29, 28},
		/*  8*/ {31, 30},
		/*  9*/ {2, 28},
		/* 10*/ {34, 33},
		/* 11*/ {12, 11},
		/* 12*/ {32, 1, 32, 36},
		/* 13*/ {13, 7, 5},
		/* 14*/ {32, 1, 32, 5, 37},
		/* 15*/ {35, 3, 5, 38},
		/* 16*/ {32, 29, 39},
		/* 17*/ {2, 31, 30},
		/* 18*/ {42, 3, 44},
		/* 19*/ {43, 3, 45},
		/* 20*/ {42, 3, 50},
		/* 21*/ {43, 3, 51},
		/* 22*/ {43, 3, 52},
		/* 23*/ {16, 15, 54},
		/* 24*/ {23, 55},
		/* 25*/ {56, 7, 46},
		/* 26*/ {56, 7, 48},
		/* 27*/ {57, 7, 17},
		/* 28*/ {18, 40, 59},
		/* 29*/ {47, 7, 49},
		/* 30*/ {53, 7, 41},
		/* 31*/ {58, 27},
		/* 32*/ {23, 31, 30},
	},
	forms: []sveForm{
		{mask: 0xffe0fc1f, value: 0x25a02000, op: 6, args: 8, gpr: 32},   // CTERMEQ  <R><n>, <R><m>
		{mask: 0xffe0fc1f, value: 0x25e02000, op: 7, args: 8, gpr: 64},   // CTERMEQ  <R><n>, <R><m>
		{mask: 0xff3fe3c0, value: 0x65188000, op: 10, args: 12},          // FADD  <Zdn>.<T>, <Pg>/M, <Zdn>.<T>, <const>
		{mask: 0xff3ffc10, value: 0x25207010, op: 22, args: 24},          // PEXT  <Pd>.<T>, <PNn>[<imm>]
		{mask: 0xff3ffe00, value: 0x252b8800, op: 29, args: 31},          // UQDECP  <Wdn>, <Pm>.<T>
		{mask: 0xff3ffa00, value: 0x25208200, op: 4, args: 6},            // CNTP  <Xd>, <PNn>.<T>, <vl>
		{mask: 0xffffe000, value: 0x65c02000, op: 11, args: 13},          // FADDV  <V><d>, <Pg>, <Zn>.<T>
		{mask: 0xfff0fc20, value: 0x45b02800, op: 26, args: 28},          // SQRSHRN  <Zd>.H, { <Zn1>.S-<Zn2>.S }, #<const>
		{mask: 0xff3fe010, value: 0x65122000, op: 13, args: 15},          // FCMEQ  <Pd>.<T>, <Pg>/Z, <Zn>.<T>, #0.0
		{mask: 0xff3fe000, value: 0x04000000, op: 0, args: 0},            // ADD  <Zdn>.<T>, <Pg>/M, <Zdn>.<T>, <Zm>.<T>
		{mask: 0xff3fe000, value: 0x04052000, op: 1, args: 1},            // ADDQV  <Vd>.<T>, <Pg>, <Zn>.<Tb>
		{mask: 0xffe0fc00, value: 0x04e04c00, op: 16, args: 17},          // INDEX  <Zd>.<T>, <R><n>, <R><m>
		{mask: 0xff3fe000, value: 0x04002000, op: 25, args: 27},          // SADDV  <Dd>, <Pg>, <Zn>.<T>
		{mask: 0xff3fc000, value: 0x2538c000, op: 8, args: 9},            // DUP  <Zd>.<T>, #<imm>{, <shift>}
		{mask: 0xff3ee000, value: 0x64008000, op: 12, args: 14},          // FCADD  <Zdn>.<T>, <Pg>/M, <Zdn>.<T>, <Zm>.<T>, <const>
		{mask: 0xff20fc10, value: 0x25201400, op: 30, args: 32, gpr: 64}, // WHILELT  <Pd>.<T>, <R><n>, <R><m>
		{mask: 0xffe0f000, value: 0x0420a000, op: 2, args: 3},            // ADR  <Zd>.D, [<Zn>.D, <Zm>.D, SXTW{<amount>}]
		{mask: 0xff20fc00, value: 0x04209000, op: 3, args: 4},            // ASR  <Zd>.<T>, <Zn>.<T>, #<const>
		{mask: 0xff20fc00, value: 0x05202000, op: 8, args: 10},           // DUP  <Zd>.<T>, <Zn>.<T>[<imm>]
		{mask: 0xffe0f000, value: 0x64e01000, op: 14, args: 5},           // FCMLA  <Zda>.S, <Zn>.S, <Zm>.S[<imm>], <const>
		{mask: 0xff20fc00, value: 0x04209c00, op: 21, args: 23},          // LSL  <Zd>.<T>, <Zn>.<T>, #<const>
		{mask: 0xffe0e010, value: 0x8400e000, op: 23, args: 25},          // PRFB  <prfop>, <Pg>, [<Zn>.S{, #<imm>}]
		{mask: 0xffe0e010, value: 0x8580c000, op: 24, args: 26},          // PRFD  <prfop>, <Pg>, [<Xn|SP>, <Xm>, LSL #3]
		{mask: 0xfff0e000, value: 0xe430e000, op: 28, args: 30},          // ST2B  { <Zt1>.B, <Zt2>.B }, <Pg>, [<Xn|SP>{, #<imm>, MUL VL}]
		{mask: 0xffa0f000, value: 0x04a0a000, op: 2, args: 2},            // ADR  <Zd>.<T>, [<Zn>.<T>, <Zm>.<T>{, <mod> <amount>}]
		{mask: 0xfffc0000, value: 0x05c00000, op: 9, args: 11},           // DUPM  <Zd>.<T>, #<const>
		{mask: 0xffe0e000, value: 0xa4004000, op: 17, args: 18},          // LD1B  { <Zt>.B }, <Pg>/Z, [<Xn|SP>, <Xm>]
		{mask: 0xffe0e000, value: 0x8520c000, op: 19, args: 22},          // LD1W  { <Zt>.S }, <Pg>/Z, [<Zn>.S{, #<imm>}]
		{mask: 0xffe0e000, value: 0xa500c000, op: 20, args: 21},          // LDNT1W  { <Zt>.S }, <Pg>/Z, [<Xn|SP>, <Xm>, LSL #2]
		{mask: 0xffe0e000, value: 0xe4202000, op: 27, args: 29},          // ST1Q  { <Zt>.Q }, <Pg>, [<Zn>.D{, <Xm>}]
		{mask: 0xff30e000, value: 0x0510c000, op: 15, args: 16},          // FCPY  <Zd>.<T>, <Pg>/M, #<const>
		{mask: 0xffa0e000, value: 0x84004000, op: 17, args: 19},          // LD1B  { <Zt>.S }, <Pg>/Z, [<Xn|SP>, <Zm>.S, <mod>]
		{mask: 0xffc0e000, value: 0x84408000, op: 18, args: 20},          // LD1RB  { <Zt>.B }, <Pg>/Z, [<Xn|SP>{, #<imm>}]
		{mask: 0xff30c000, value: 0x05104000, op: 5, args: 7},            // CPY  <Zd>.<T>, <Pg>/M, #<imm>{, <shift>}
	},
}

// sveTestWords are decoded through Decode, GNUSyntax and GoSyntax. The GNU
// text is what llvm-mc 22 prints with -M no-aliases. There are two exceptions.
// LLVM prints DUPM with the immediate replicated to 64 bits, so those rows are
// GNU objdump 2.45's.
// And every disassembler spells a floating-point immediate differently (#2.0,
// #2.00000000, #2.000000000000000000e+00), so those rows use the one spelling
// Imm_fp has, which is also what the existing NEON FMOV tests expect. The Go text
// is the testdata line that pins the word in cmd/asm, or for dupm z3.b a line that
// cmd/asm assembles to it. None of it is this decoder's own output. Like
// decode_test.go, the comparison ignores spaces.
var sveTestWords = []struct {
	word  uint32
	gnu   string
	plan9 string
}{
	{0x25bf2320, "ctermeq w25, wzr", "CTERMEQW ZR, R25"},
	{0x25ff2320, "ctermeq x25, xzr", "CTERMEQ ZR, R25"},
	{0x65d89425, "fadd z5.d, p5/m, z5.d, #1.000000000000000000e+00", "ZFADD $(1.0), Z5.D, P5.M, Z5.D"},
	{0x25a0717d, "pext p13.s, pn11[1]", "PPEXT PN11[1], P13.S"},
	{0x25ab89df, "uqdecp wzr, p14.s", "PUQDECPW P14.S, ZR"},
	{0x25e083a5, "cntp x5, pn13.d, vlx2", "PCNTP VLx2, PN13.D, R5"},
	{0x65c02d4f, "faddv d15, p3, z10.d", "ZFADDVD Z10.D, P3, V15"},
	{0x45ba284a, "sqrshrn z10.h, { z2.s, z3.s }, #6", "ZSQRSHRN $6, [Z2.S-Z3.S], Z10.H"},
	{0x65d230e2, "fcmeq p2.d, p4/z, z7.d, #0.0", "ZFCMEQ $(0.0), Z7.D, P4.Z, P2.D"},
	{0x04000de0, "add z0.b, p3/m, z0.b, z15.b", "ZADD Z15.B, Z0.B, P3.M, Z0.B"},
	{0x04852f25, "addqv v5.4s, p3, z25.s", "ZADDQV Z25.S, P3, V5.S4"},
	{0x04ed4e89, "index z9.d, x20, x13", "ZINDEX R13, R20, Z9.D"},
	{0x04002cc2, "saddv d2, p3, z6.b", "ZSADDVD Z6.B, P3, V2"},
	{0x2538c12b, "dup z11.b, #9", "ZDUP $9, Z11.B"},
	{0x648091a4, "fcadd z4.s, p4/m, z4.s, z13.s, #90", "ZFCADD $90, Z13.S, Z4.S, P4.M, Z4.S"},
	{0x2562154a, "whilelt p10.h, x10, x2", "PWHILELT R2, R10, P10.H"},
	{0x0427a8d7, "adr z23.d, [z6.d, z7.d, sxtw #2]", "ZADR (Z7.D.SXTW<<2)(Z6.D), Z23.D"},
	{0x04fa90f7, "asr z23.d, z7.d, #6", "ZASR $6, Z7.D, Z23.D"},
	{0x05702379, "dup z25.q, z27.q[1]", "ZDUP Z27.Q[1], Z25.Q"},
	{0x64e31c57, "fcmla z23.s, z2.s, z3.s[0], #270", "ZFCMLA $270, Z3.S[0], Z2.S, Z23.S"},
	{0x04a69cf7, "lsl z23.d, z7.d, #6", "ZLSL $6, Z7.D, Z23.D"},
	{0x8408ed4c, "prfb pstl3keep, p3, [z10.s, #8]", "ZPRFB 8(Z10.S), P3, PSTL3KEEP"},
	{0x8588cfec, "prfd pstl3keep, p3, [sp, x8, lsl #3]", "PPRFD (R8<<3)(RSP), P3, PSTL3KEEP"},
	{0xe43fefe8, "st2b { z8.b, z9.b }, p3, [sp, #-2, mul vl]", "ZST2B [Z8.B, Z9.B], P3, (-VL*2)(RSP)"},
	{0x04b0a18d, "adr z13.s, [z12.s, z16.s]", "ZADR (Z16.S)(Z12.S), Z13.S"},
	{0x05c00446, "dupm z6.h, #0x7", "ZDUPM $7, Z6.H"},
	{0x05c00783, "dupm z3.b, #0x55", "ZDUPM $85, Z3.B"},
	{0xa40651cd, "ld1b { z13.b }, p4/z, [x14, x6]", "ZLD1B (R6)(R14), P4.Z, [Z13.B]"},
	{0x8523d819, "ld1w { z25.s }, p6/z, [z0.s, #12]", "ZLD1W 12(Z0.S), P6.Z, [Z25.S]"},
	{0xa506d1cd, "ldnt1w { z13.s }, p4/z, [x14, x6, lsl #2]", "ZLDNT1W (R6<<2)(R14), P4.Z, [Z13.S]"},
	{0xe4262de8, "st1q { z8.q }, p3, [z15.d, x6]", "ZST1Q [Z8.Q], P3, (R6)(Z15.D)"},
	{0x0556c017, "fcpy z23.h, p6/m, #2.000000000000000000e+00", "ZFCPY $(2.0), P6.M, Z23.H"},
	{0x84044c64, "ld1b { z4.s }, p3/z, [x3, z4.s, uxtw]", "ZLD1B (Z4.S.UXTW)(R3), P3.Z, [Z4.S]"},
	{0x844691cd, "ld1rb { z13.b }, p4/z, [x14, #6]", "ZLD1RB 6(R14), P4.Z, [Z13.B]"},
	{0x055540c6, "cpy z6.h, p5/m, #6", "ZCPY $6, P5.M, Z6.H"},
}

func noSpace(s string) string { return strings.ReplaceAll(s, " ", "") }

func decodeWord(w uint32) (Inst, error) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], w)
	return Decode(b[:])
}

func TestSVEDecodeWords(t *testing.T) {
	useSVETables(t, sveTestTables)
	for _, tt := range sveTestWords {
		inst, err := decodeWord(tt.word)
		if err != nil {
			t.Errorf("Decode(%#08x) = %v, want %q", tt.word, err, tt.gnu)
			continue
		}
		if inst.Enc != tt.word {
			t.Errorf("Decode(%#08x).Enc = %#08x", tt.word, inst.Enc)
		}
		if got := GNUSyntax(inst); noSpace(got) != noSpace(tt.gnu) {
			t.Errorf("Decode(%#08x) in GNU syntax = %q, want %q", tt.word, got, tt.gnu)
		}
		if got := GoSyntax(inst, 0, nil, nil); got != tt.plan9 {
			t.Errorf("Decode(%#08x) in Go syntax = %q, want %q", tt.word, got, tt.plan9)
		}
	}
}

// TestSVESampleTables decodes with the tables in sve_sample_gen_test.go, which
// instgen's TestSampleDecoderTables generates from its own test data, the
// unpredicated SVE ADD. Together the two tests take one instruction from the
// ARM XML to its disassembly.
func TestSVESampleTables(t *testing.T) {
	useSVETables(t, sveTables{sveSampleOpARM, sveSampleOpGo, sveSampleSyms, sveSampleDescs,
		sveSampleArgs, sveSampleArgLists, sveSampleForms})
	for _, tt := range []struct {
		word  uint32
		gnu   string
		plan9 string
	}{
		{0x04e702ed, "add z13.d, z23.d, z7.d", "ZADD Z7.D, Z23.D, Z13.D"},
		{0x04220020, "add z0.b, z1.b, z2.b", "ZADD Z2.B, Z1.B, Z0.B"},
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], tt.word)
		inst, err := Decode(b[:])
		if err != nil {
			t.Errorf("Decode(%#08x): %v", tt.word, err)
			continue
		}
		if got := GNUSyntax(inst); got != tt.gnu {
			t.Errorf("Decode(%#08x) in GNU syntax = %q, want %q", tt.word, got, tt.gnu)
		}
		if got := GoSyntax(inst, 0, nil, nil); got != tt.plan9 {
			t.Errorf("Decode(%#08x) in Go syntax = %q, want %q", tt.word, got, tt.plan9)
		}
	}
	// The sample has only the unpredicated ADD, so neither the predicated ADD
	// nor the unpredicated SUB, which differ from it in bit 21 and bit 10,
	// decodes.
	for _, w := range []uint32{0x04000000, 0x04200400} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], w)
		if inst, err := Decode(b[:]); err == nil {
			t.Errorf("Decode(%#08x) = %v, want an error: the sample has only the unpredicated ADD", w, inst)
		}
	}
}

// TestSVEDecodeEdges covers spellings that the pinned words do not reach: the
// registers that the Go assembler renames, register 31 as a stack pointer and as
// the zero register, a register list that wraps, and the extremes of an offset.
// The GNU text is llvm-mc 22's with -M no-aliases. The Go text follows the
// pinned words' spelling, with R18 and R28 written as the assembler requires.
func TestSVEDecodeEdges(t *testing.T) {
	useSVETables(t, sveTestTables)
	for _, tt := range []struct {
		name  string
		word  uint32
		gnu   string
		plan9 string
	}{
		{"R18 and R28 are renamed", 0x25fc2240, "ctermeq x18, x28", "CTERMEQ g, R18_PLATFORM"},
		{"31 is the zero register", 0x25ff23e0, "ctermeq xzr, xzr", "CTERMEQ ZR, ZR"},
		{"index is R18", 0xa41251cd, "ld1b { z13.b }, p4/z, [x14, x18]", "ZLD1B (R18_PLATFORM)(R14), P4.Z, [Z13.B]"},
		{"index is R28", 0xa41c51cd, "ld1b { z13.b }, p4/z, [x14, x28]", "ZLD1B (g)(R14), P4.Z, [Z13.B]"},
		{"31 as a base is the stack pointer", 0xa40653ed, "ld1b { z13.b }, p4/z, [sp, x6]", "ZLD1B (R6)(RSP), P4.Z, [Z13.B]"},
		{"negative immediate", 0x2538dfeb, "dup z11.b, #-1", "ZDUP $-1, Z11.B"},
		{"most negative immediate", 0x2538d00b, "dup z11.b, #-128", "ZDUP $-128, Z11.B"},
		// cmd/asm has names for the twelve named prefetch operations only, so the
		// Go text of these four is readable but cannot be assembled.
		{"numeric prefetch operation", 0x8588cfe6, "prfd #6, p3, [sp, x8, lsl #3]", "PPRFD (R8<<3)(RSP), P3, $6"},
		{"last numeric prefetch operation", 0x8588cfef, "prfd #15, p3, [sp, x8, lsl #3]", "PPRFD (R8<<3)(RSP), P3, $15"},
		{"a list wraps from Z31 to Z0", 0xe43fefff, "st2b { z31.b, z0.b }, p3, [sp, #-2, mul vl]", "ZST2B [Z31.B, Z0.B], P3, (-VL*2)(RSP)"},
		{"a list ends at Z31", 0xe43feffe, "st2b { z30.b, z31.b }, p3, [sp, #-2, mul vl]", "ZST2B [Z30.B, Z31.B], P3, (-VL*2)(RSP)"},
		{"largest MUL VL offset", 0xe437efe8, "st2b { z8.b, z9.b }, p3, [sp, #14, mul vl]", "ZST2B [Z8.B, Z9.B], P3, (VL*14)(RSP)"},
		{"smallest MUL VL offset", 0xe438efe8, "st2b { z8.b, z9.b }, p3, [sp, #-16, mul vl]", "ZST2B [Z8.B, Z9.B], P3, (-VL*16)(RSP)"},
	} {
		inst, err := decodeWord(tt.word)
		if err != nil {
			t.Errorf("%s: Decode(%#08x) = %v", tt.name, tt.word, err)
			continue
		}
		if got := GNUSyntax(inst); noSpace(got) != noSpace(tt.gnu) {
			t.Errorf("%s: GNU syntax = %q, want %q", tt.name, got, tt.gnu)
		}
		if got := GoSyntax(inst, 0, nil, nil); got != tt.plan9 {
			t.Errorf("%s: Go syntax = %q, want %q", tt.name, got, tt.plan9)
		}
	}
}

// TestSVEDecodeDeclines checks words that match a form's mask and value but
// that one of its operands cannot resolve. llvm-mc 22 calls each of them an
// invalid encoding. Words that the descriptors accept but the architecture
// forbids, such as a reserved register number, are for sveReject.
func TestSVEDecodeDeclines(t *testing.T) {
	useSVETables(t, sveTestTables)
	for _, tt := range []struct {
		name string
		word uint32
	}{
		{"element size field is zero in a shift", 0x042290f7},
		{"element size field is zero in a narrowing shift", 0x4522284a},
		{"element size field is zero in a left shift", 0x04269cf7},
		{"logical immediate with all bits set", 0x05c3ffe6},
	} {
		if inst, err := decodeWord(tt.word); err == nil {
			t.Errorf("%s: Decode(%#08x) = %v, want an error", tt.name, tt.word, inst)
		}
	}
}

// TestSVEDecodeWithoutTables checks that the runtime is inert when no tables are
// installed, so a stripped-down build of the package behaves as the stock
// decoder does.
func TestSVEDecodeWithoutTables(t *testing.T) {
	useSVETables(t, sveTables{})
	for _, tt := range sveTestWords {
		if inst, err := decodeWord(tt.word); err != errUnknown {
			t.Errorf("Decode(%#08x) = %v, %v, want errUnknown", tt.word, inst, err)
		}
	}
	if sveIsOp(sveOpBase) {
		t.Errorf("sveIsOp(sveOpBase) with no tables")
	}
}

// TestSVEDecodeNeonUnchanged checks that installing SVE tables moves no existing
// answer: stock words decode to the same text with and without them.
func TestSVEDecodeNeonUnchanged(t *testing.T) {
	words := []uint32{
		0x8b020020, // add x0, x1, x2
		0x4ea18400, // add v0.4s, v0.4s, v1.4s
		0x4e20d400, // fadd v0.4s, v0.4s, v0.4s
		0x0e000400, // dup v0.8b, v0.b[0]
		0xd503201f, // nop
		0xa9bf7bfd, // stp x29, x30, [sp, #-16]!
	}
	decode := func() []string {
		var out []string
		for _, w := range words {
			inst, err := decodeWord(w)
			out = append(out, fmt.Sprint(GNUSyntax(inst), "|", GoSyntax(inst, 0, nil, nil), "|", err))
		}
		return out
	}
	want := decode()
	useSVETables(t, sveTestTables)
	for i, got := range decode() {
		if got != want[i] {
			t.Errorf("Decode(%#08x) with SVE tables = %q, without = %q", words[i], got, want[i])
		}
	}
}

func TestSVEReject(t *testing.T) {
	tables := sveTestTables
	tables.forms = append([]sveForm(nil), tables.forms...)
	for i := range tables.forms {
		if tables.forms[i].op == 0 { // ADD
			tables.forms[i].reject = 7
		}
	}
	useSVETables(t, tables)
	const word = 0x04000de0 // add z0.b, p3/m, z0.b, z15.b
	if _, err := decodeWord(word); err != nil {
		t.Fatalf("Decode(%#08x) = %v", word, err)
	}
	old := sveReject
	t.Cleanup(func() { sveReject = old })
	var seen []int
	sveReject = func(class int, w uint32) bool {
		seen = append(seen, class)
		return w == word
	}
	if inst, err := decodeWord(word); err == nil {
		t.Errorf("Decode(%#08x) = %v with the form rejected, want an error", word, inst)
	}
	if len(seen) == 0 || seen[0] != 7 {
		t.Errorf("sveReject was called with classes %v, want 7 first", seen)
	}
	if _, err := decodeWord(word ^ 1<<5); err != nil {
		t.Errorf("Decode of a different word = %v, want it unaffected", err)
	}
}

func TestSVEOps(t *testing.T) {
	if sveOpBase < Op(len(opstr)) {
		t.Fatalf("sveOpBase = %d overlaps the %d stock ops", sveOpBase, len(opstr))
	}
	useSVETables(t, sveTestTables)
	for i, name := range sveTestTables.opARM {
		op := sveOpBase + Op(i)
		if !sveIsOp(op) {
			t.Errorf("sveIsOp(%d) = false", op)
		}
		if got := op.String(); got != name {
			t.Errorf("Op(%d).String() = %q, want %q", op, got, name)
		}
	}
	past := sveOpBase + Op(len(sveTestTables.opARM))
	if sveIsOp(past) || sveIsOp(sveOpBase-1) {
		t.Errorf("sveIsOp is true outside the table")
	}
	if got, want := past.String(), fmt.Sprintf("Op(%d)", int(past)); got != want {
		t.Errorf("Op(%d).String() = %q, want %q", past, got, want)
	}
	if got := ADD.String(); got != "ADD" {
		t.Errorf("ADD.String() = %q", got)
	}
}

// TestSVETestTablesCoverRuntime checks that sveTestTables uses every operand
// kind, every register file and every knob, so that a case added to the runtime
// without a form that reaches it shows up here.
func TestSVETestTablesCoverRuntime(t *testing.T) {
	tab := sveTestTables
	kinds := map[kind]bool{}
	for _, a := range tab.args {
		kinds[a.kind] = true
	}
	for k := kArng; k <= kMemVecIndex; k++ {
		if !kinds[k] {
			t.Errorf("no form uses operand kind %d", k)
		}
	}
	roles := map[role]bool{}
	files := map[regfile]bool{}
	tsz := map[tszMode]bool{}
	knobs := map[string]bool{}
	for _, d := range tab.descs {
		roles[d.role] = true
		files[d.regfile] = true
		tsz[d.tsz] = true
		for name, used := range map[string]bool{
			"signed":    d.signed,
			"range":     d.hasRange,
			"stride":    d.stride != 0,
			"bias":      d.bias != 0,
			"modulo":    d.modulo != 0,
			"sp":        d.sp,
			"alt":       d.altBit != 0,
			"fixed":     d.fixed >= 0,
			"table":     len(d.table) != 0,
			"fp8":       d.fp8,
			"bitmask":   d.bitmask,
			"vwidth":    d.vwidth != 0,
			"invert":    d.invert != 0,
			"multipart": len(d.fields) > 1,
		} {
			knobs[name] = knobs[name] || used
		}
	}
	for r := roleReg; r <= rolePrfop; r++ {
		if !roles[r] {
			t.Errorf("no descriptor has role %d", r)
		}
	}
	for f := rfZ; f <= rfV; f++ {
		if !files[f] {
			t.Errorf("no descriptor names register file %d", f)
		}
	}
	for m := tszIndex; m <= tszShiftL; m++ {
		if !tsz[m] {
			t.Errorf("no descriptor uses element-size mode %d", m)
		}
	}
	for _, name := range []string{"signed", "range", "stride", "bias", "modulo", "sp", "alt",
		"fixed", "table", "fp8", "bitmask", "vwidth", "invert", "multipart"} {
		if !knobs[name] {
			t.Errorf("no descriptor uses knob %q", name)
		}
	}
	for _, f := range tab.forms {
		if strings.HasPrefix(tab.opARM[f.op], "ST") {
			return
		}
	}
	t.Errorf("no form is a store")
}

func TestSVEDescValue(t *testing.T) {
	for _, tt := range []struct {
		name string
		d    sveDesc
		word uint32
		want int64
		ok   bool
	}{
		{"plain", sveDesc{fields: []sveField{{5, 5}}}, 0x1f << 5, 31, true},
		{"low part first", sveDesc{fields: []sveField{{16, 3}, {22, 2}}}, 5<<16 | 2<<22, 5 | 2<<3, true},
		{"signed negative", sveDesc{fields: []sveField{{5, 6}}, signed: true}, 0x3f << 5, -1, true},
		{"signed most negative", sveDesc{fields: []sveField{{5, 6}}, signed: true}, 0x20 << 5, -32, true},
		{"signed positive", sveDesc{fields: []sveField{{5, 6}}, signed: true}, 0x1f << 5, 31, true},
		{"signed across parts", sveDesc{fields: []sveField{{0, 2}, {10, 2}}, signed: true}, 3<<10 | 1, -3, true},
		{"stride", sveDesc{fields: []sveField{{16, 4}}, stride: 2}, 3 << 16, 6, true},
		{"stride and bias", sveDesc{fields: []sveField{{1, 3}}, stride: 2, bias: 1}, 3 << 1, 7, true},
		{"bias", sveDesc{fields: []sveField{{0, 3}}, hasRange: true, min: 8, max: 15, bias: 8}, 5, 13, true},
		{"modulo wraps", sveDesc{fields: []sveField{{0, 5}}, bias: 1, modulo: 32}, 31, 0, true},
		{"modulo in range", sveDesc{fields: []sveField{{0, 5}}, bias: 1, modulo: 32}, 4, 5, true},
		{"range ok at the top", sveDesc{fields: []sveField{{5, 4}}, hasRange: true, max: 14}, 14 << 5, 14, true},
		{"modulo of a negative value", sveDesc{fields: []sveField{{0, 3}}, signed: true, modulo: 8}, 7, 7, true},
		{"range exceeded", sveDesc{fields: []sveField{{5, 4}}, hasRange: true, max: 14}, 15 << 5, 0, false},
		{"range below", sveDesc{fields: []sveField{{0, 3}}, hasRange: true, min: 2, max: 7}, 1, 0, false},
		{"alternative shift off", sveDesc{fields: []sveField{{5, 8}}, altBit: 13, altShift: 8}, 9 << 5, 9, true},
		{"alternative shift on", sveDesc{fields: []sveField{{5, 8}}, altBit: 13, altShift: 8}, 9<<5 | 1<<13, 9 << 8, true},
		// The range applies to the field and the shift scales it afterwards.
		{"range before alternative shift", sveDesc{fields: []sveField{{5, 8}}, signed: true, hasRange: true, min: -128, max: 127, altBit: 13, altShift: 8}, 0xff<<5 | 1<<13, -1 << 8, true},
		{"invert", sveDesc{fields: []sveField{{16, 4}}, hasRange: true, min: 1, max: 16, invert: 16}, 3 << 16, 13, true},
		{"invert out of range", sveDesc{fields: []sveField{{16, 4}}, hasRange: true, min: 1, max: 15, invert: 16}, 0, 0, false},
		// The range applies to the inverted value, and both ends are in it.
		{"invert at the top of the range", sveDesc{fields: []sveField{{16, 3}}, hasRange: true, min: 1, max: 8, invert: 8}, 0, 8, true},
		{"invert at the bottom of the range", sveDesc{fields: []sveField{{16, 3}}, hasRange: true, min: 1, max: 7, invert: 8}, 7 << 16, 1, true},
	} {
		got, ok := tt.d.value(tt.word)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: value(%#x) = %d, %v, want %d, %v", tt.name, tt.word, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSVEDescEsizeValue(t *testing.T) {
	// The field is a 7-bit composite whose low bits select the element size.
	d := func(m tszMode) *sveDesc { return &sveDesc{fields: []sveField{{0, 7}}, tsz: m} }
	for _, tt := range []struct {
		name  string
		m     tszMode
		field uint32
		esize int64
		want  int64
		ok    bool
	}{
		// Index: what remains above the size-selecting bits.
		{"index in bytes", tszIndex, 0b0101011, 8, 0b010101, true},
		{"index in halfwords", tszIndex, 0b0010110, 16, 0b00101, true},
		{"index in words", tszIndex, 0b0001100, 32, 0b0001, true},
		{"index in doublewords", tszIndex, 0b0111000, 64, 0b011, true},
		{"index in quadwords", tszIndex, 0b1110000, 128, 0b11, true},
		// Right shift: 2*esize minus the field, in 1..esize.
		{"right shift by one", tszShiftR, 2*16 - 1, 16, 1, true},
		{"right shift by esize", tszShiftR, 16, 16, 16, true},
		{"right shift too large", tszShiftR, 15, 16, 0, false},
		{"right shift too small", tszShiftR, 2 * 16, 16, 0, false},
		// Left shift: the field minus esize, in 0..esize-1.
		{"left shift by zero", tszShiftL, 32, 32, 0, true},
		{"left shift by esize-1", tszShiftL, 63, 32, 31, true},
		{"left shift too large", tszShiftL, 64, 32, 0, false},
		{"left shift negative", tszShiftL, 31, 32, 0, false},
		{"no element size", tszShiftL, 5, 0, 0, false},
	} {
		got, ok := d(tt.m).esizeValue(tt.field, tt.esize)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: esizeValue(%#b, %d) = %d, %v, want %d, %v", tt.name, tt.field, tt.esize, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSVEDecodeBitMasks(t *testing.T) {
	// Expected values follow DecodeBitMasks in the Arm ARM. The result is one
	// lane, which is what the Go assembler takes as the operand: an element of
	// 32 bits or less is not repeated to 64 bits, but one of 2 or 4 bits is
	// repeated to fill a byte. GNU objdump 2.45 agrees on the 2- and 4-bit rows:
	// 0x05800f80, with immr = 1 and imms = 111100, is and z0.b, z0.b, #0xaa.
	for _, tt := range []struct {
		n, immr, imms uint32
		want          uint64
		ok            bool
	}{
		{0, 0b000000, 0b111100, 0x55, true},
		{0, 0b000001, 0b111100, 0xaa, true},
		{0, 0b111111, 0b111100, 0xaa, true},
		{0, 0b000000, 0b111000, 0x11, true},
		{0, 0b000001, 0b111001, 0x99, true},
		{0, 0b000001, 0b111010, 0xbb, true},
		{0, 0b000011, 0b111010, 0xee, true},
		{0, 0b000000, 0b111011, 0, false},
		{0, 0b000000, 0b110000, 0x1, true},
		{0, 0b000000, 0b110001, 0x3, true},
		{0, 0b000010, 0b110011, 0xc3, true},
		{1, 0b000000, 0b000000, 0x1, true},
		{1, 0b000001, 0b000000, 0x8000000000000000, true},
		{1, 0b000000, 0b111110, 0x7fffffffffffffff, true},
		{1, 0b000000, 0b111111, 0, false},
		{0, 0b000000, 0b111111, 0, false},
		{0, 0b000000, 0b000000, 0x1, true},
		{0, 0b000101, 0b100101, 0xf801, true},
		{0, 0b000011, 0b001010, 0xe00000ff, true},
		{0, 0b000000, 0b111110, 0, false},
		{0, 0b000000, 0b111101, 0, false},
		{1, 0b111111, 0b000101, 0x7e, true},
		{0, 0b000100, 0b000111, 0xf000000f, true},
	} {
		got, ok := decodeBitMasks(tt.n<<12 | tt.immr<<6 | tt.imms)
		if got != tt.want || ok != tt.ok {
			t.Errorf("decodeBitMasks(N=%d immr=%#b imms=%#b) = %#x, %v, want %#x, %v", tt.n, tt.immr, tt.imms, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSVEDescReg(t *testing.T) {
	d := func(f regfile, lo, width int) *sveDesc {
		return &sveDesc{fields: []sveField{{lo, width}}, regfile: f, fixed: -1}
	}
	for _, tt := range []struct {
		name     string
		d        *sveDesc
		word     uint32
		esize    int64
		gprWidth int64
		want     Reg
		ok       bool
	}{
		{"Z", d(rfZ, 5, 5), 31 << 5, 0, 0, Z31, true},
		{"P", d(rfP, 10, 4), 15 << 10, 0, 0, P15, true},
		{"PN", d(rfPN, 5, 4), 8 << 5, 0, 0, PN8, true},
		{"R32", d(rfR32, 0, 5), 30, 0, 0, W30, true},
		{"R64", d(rfR64, 0, 5), 30, 0, 0, X30, true},
		{"R64 register 31 is the zero register", d(rfR64, 0, 5), 31, 0, 0, XZR, true},
		{"R32 register 31", d(rfR32, 0, 5), 31, 0, 0, WZR, true},
		{"automatic width from a 64-bit element", d(rfRauto, 0, 5), 3, 64, 0, X3, true},
		{"automatic width from a 32-bit element", d(rfRauto, 0, 5), 3, 32, 0, W3, true},
		{"automatic width from a 16-bit element", d(rfRauto, 0, 5), 3, 16, 0, W3, true},
		{"automatic width pinned to 64", d(rfRauto, 0, 5), 3, 8, 64, X3, true},
		{"automatic width pinned to 32", d(rfRauto, 0, 5), 3, 64, 32, W3, true},
		{"V from a byte element", d(rfV, 0, 5), 7, 8, 0, B7, true},
		{"V from a halfword element", d(rfV, 0, 5), 7, 16, 0, H7, true},
		{"V from a word element", d(rfV, 0, 5), 7, 32, 0, S7, true},
		{"V from a doubleword element", d(rfV, 0, 5), 7, 64, 0, D7, true},
		{"V from a quadword element", d(rfV, 0, 5), 7, 128, 0, Q7, true},
		{"V with no element size", d(rfV, 0, 5), 7, 0, 0, V7, true},
		{"V with a stated width", &sveDesc{fields: []sveField{{0, 5}}, regfile: rfV, vwidth: 64, fixed: -1}, 7, 8, 0, D7, true},
		{"negative", &sveDesc{fields: []sveField{{0, 3}}, signed: true, regfile: rfZ, fixed: -1}, 7, 0, 0, 0, false},
		{"no file", d(rfNone, 0, 5), 7, 0, 0, 0, false},
		// The top of each file is a register, and the number after it is not.
		{"PN15", d(rfPN, 5, 4), 15 << 5, 0, 0, PN15, true},
		{"V31 as a quadword", d(rfV, 0, 5), 31, 128, 0, Q31, true},
		{"R64 31", d(rfR64, 0, 5), 31, 0, 0, XZR, true},
		{"Z32", d(rfZ, 0, 6), 32, 0, 0, 0, false},
		{"P16", d(rfP, 0, 5), 16, 0, 0, 0, false},
		{"PN16", d(rfPN, 0, 5), 16, 0, 0, 0, false},
		{"V32", d(rfV, 0, 6), 32, 8, 0, 0, false},
		{"R32 32", d(rfR32, 0, 6), 32, 0, 0, 0, false},
		{"R64 32", d(rfR64, 0, 6), 32, 0, 0, 0, false},
		{"automatic width 32", d(rfRauto, 0, 6), 32, 64, 0, 0, false},
	} {
		got, ok := tt.d.reg(tt.word, tt.esize, tt.gprWidth)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: reg = %v, %v, want %v, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// A descriptor whose role the runtime does not know must make the operand
// fail, not resolve to whatever the other descriptors give.
func TestSVEResolveUnknownRole(t *testing.T) {
	useSVETables(t, sveTables{descs: []sveDesc{
		{role: roleReg, fields: []sveField{{0, 5}}, regfile: rfZ, fixed: -1},
		{role: roleSkip, fixed: -1},
		{role: role(200), fixed: -1},
	}})
	if _, ok := sveResolve(&sveArg{descs: []uint16{0}}, 3, 0, 0); !ok {
		t.Fatal("a register descriptor alone does not resolve")
	}
	for _, di := range []uint16{1, 2} {
		if p, ok := sveResolve(&sveArg{descs: []uint16{0, di}}, 3, 0, 0); ok {
			t.Errorf("a descriptor of role %d resolved: %+v", sveDescs[di].role, p)
		}
	}
}

// A descriptor that cannot read its part of the word fails the whole operand:
// the next check does not have to notice that something is missing.
func TestSVEResolveDeclines(t *testing.T) {
	useSVETables(t, sveTables{
		syms: []string{"B", "foo", "LSL", "#1"},
		descs: []sveDesc{
			/* 0*/ {role: roleReg, fields: []sveField{{0, 6}}, regfile: rfZ, fixed: -1},
			/* 1*/ {role: roleArng, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 0}}},
			/* 2*/ {role: roleArng, fixed: 1},
			/* 3*/ {role: roleQual, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 0}}},
			/* 4*/ {role: roleQual, fixed: 1},
			/* 5*/ {role: roleMod, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 2}}},
			/* 6*/ {role: roleMod, fixed: 1},
			/* 7*/ {role: roleAmount, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 3}}},
			/* 8*/ {role: roleAmount, fixed: 1},
			/* 9*/ {role: roleVL, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 3}}},
			/*10*/ {role: roleImm, fields: []sveField{{22, 2}}, fixed: -1, table: []sveRow{{0x3, 0x0, 3}}},
			/*11*/ {role: roleImm, fields: []sveField{{0, 3}}, hasRange: true, min: 0, max: 3, fixed: -1},
			/*12*/ {role: roleImm, fields: []sveField{{0, 13}}, bitmask: true, fixed: -1},
			/*13*/ {role: roleImm, fields: []sveField{{0, 7}}, tsz: tszShiftR, fixed: -1},
			/*14*/ {role: roleIndex, fields: []sveField{{0, 3}}, hasRange: true, min: 0, max: 3, fixed: -1},
		},
	})
	for _, tt := range []struct {
		name string
		desc uint16
		word uint32
		ok   bool
	}{
		{"register in range", 0, 31, true},
		{"register out of range", 0, 32, false},
		{"arrangement row", 1, 0, true},
		{"arrangement without a row", 1, 1 << 22, false},
		{"arrangement that is not one", 2, 0, false},
		{"qualifier row", 3, 0, true},
		{"qualifier without a row", 3, 1 << 22, false},
		{"qualifier that is not one", 4, 0, false},
		{"modifier row", 5, 0, true},
		{"modifier without a row", 5, 1 << 22, false},
		{"modifier that is not one", 6, 0, false},
		{"amount row", 7, 0, true},
		{"amount without a row", 7, 1 << 22, false},
		{"amount that is not a number", 8, 0, false},
		{"vector length row", 9, 0, true},
		{"vector length without a row", 9, 1 << 22, false},
		{"immediate row", 10, 0, true},
		{"immediate without a row", 10, 1 << 22, false},
		{"immediate in range", 11, 3, true},
		{"immediate out of range", 11, 4, false},
		{"logical immediate", 12, 0x3c, true},
		{"reserved logical immediate", 12, 0x1fff, false},
		{"shift with no element size", 13, 0, false},
		{"index in range", 14, 3, true},
		{"index out of range", 14, 4, false},
	} {
		_, ok := sveResolve(&sveArg{descs: []uint16{tt.desc}}, tt.word, 8, 0)
		if ok != tt.ok {
			t.Errorf("%s: sveResolve ok = %v, want %v", tt.name, ok, tt.ok)
		}
	}
}

// Each kind of operand needs certain parts of its descriptors. An operand that
// lacks one declines; the same kind with all of them builds.
func TestSVEBuildDeclines(t *testing.T) {
	useSVETables(t, sveTables{
		syms: []string{"B"},
		descs: []sveDesc{
			/* 0*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfZ, fixed: -1},
			/* 1*/ {role: roleReg, fields: []sveField{{5, 5}}, regfile: rfZ, fixed: -1},
			/* 2*/ {role: roleArng, fixed: 0},
			/* 3*/ {role: roleQual, fixed: 0},
			/* 4*/ {role: roleImm, fields: []sveField{{10, 3}}, fixed: -1},
			/* 5*/ {role: roleReg, fields: []sveField{{16, 5}}, regfile: rfR64, fixed: -1},
			/* 6*/ {role: roleReg, fields: []sveField{{0, 4}}, regfile: rfP, fixed: -1},
			/* 7*/ {role: roleReg, fields: []sveField{{0, 5}}, regfile: rfR64, fixed: -1},
		},
	})
	for _, tt := range []struct {
		name  string
		kind  kind
		descs []uint16
		ok    bool
	}{
		{"arrangement", kArng, []uint16{0, 2}, true},
		{"arrangement without one", kArng, []uint16{0}, false},
		{"arrangement of two registers", kArng, []uint16{0, 1, 2}, false},
		{"arrangement without a register", kArng, []uint16{2}, false},
		{"register", kReg, []uint16{0}, true},
		{"register of none", kReg, nil, false},
		{"register of two", kReg, []uint16{0, 1}, false},
		{"predicate with qualifier", kPredQual, []uint16{6, 3}, true},
		{"predicate without qualifier", kPredQual, []uint16{6}, false},
		{"qualifier without predicate", kPredQual, []uint16{3}, false},
		{"predicate of two", kPredQual, []uint16{6, 6, 3}, false},
		{"immediate", kImm, []uint16{4}, true},
		{"immediate of nothing", kImm, nil, false},
		{"indexed arrangement", kArngIdx, []uint16{0, 2, 4}, true},
		{"indexed arrangement without one", kArngIdx, []uint16{0, 4}, false},
		{"indexed arrangement without an index", kArngIdx, []uint16{0, 2}, false},
		{"indexed arrangement of two registers", kArngIdx, []uint16{0, 1, 2, 4}, false},
		{"indexed register", kRegIdx, []uint16{0, 4}, true},
		{"indexed register without an index", kRegIdx, []uint16{0}, false},
		{"indexed register of two", kRegIdx, []uint16{0, 1, 4}, false},
		{"indexed register without a register", kRegIdx, []uint16{4}, false},
		{"list", kList, []uint16{0, 1, 2}, true},
		{"list without an arrangement", kList, []uint16{0, 1}, false},
		{"list without registers", kList, []uint16{2}, false},
		{"special with a value", kSpecial, []uint16{4}, true},
		{"special with nothing", kSpecial, nil, false},
		{"memory offset", kMemOff, []uint16{5, 4}, true},
		{"memory offset without a base", kMemOff, []uint16{4}, false},
		{"memory offset of two registers", kMemOff, []uint16{5, 7, 4}, false},
		{"scaled memory offset", kMemMulVL, []uint16{5, 4}, true},
		{"scaled memory offset without a base", kMemMulVL, []uint16{4}, false},
		{"scaled memory offset of two registers", kMemMulVL, []uint16{5, 7}, false},
		{"vector base", kMemVecBase, []uint16{0, 2, 4}, true},
		{"vector base without an arrangement", kMemVecBase, []uint16{0, 4}, false},
		{"vector base of two registers", kMemVecBase, []uint16{0, 1, 2}, false},
		{"extended memory", kMemExt, []uint16{5, 7}, true},
		{"extended memory with one register", kMemExt, []uint16{5}, false},
		{"extended memory with three", kMemExt, []uint16{5, 7, 0}, false},
		{"vector index", kMemVecIndex, []uint16{5, 0}, true},
		{"vector index with one register", kMemVecIndex, []uint16{5}, false},
		{"vector index with three", kMemVecIndex, []uint16{5, 7, 0}, false},
		{"unknown kind", kind(200), []uint16{0}, false},
	} {
		if got, ok := sveBuild(&sveArg{kind: tt.kind, descs: tt.descs}, 0, 0, 0); ok != tt.ok {
			t.Errorf("%s: sveBuild = %v, %v, want ok = %v", tt.name, got, ok, tt.ok)
		}
	}
}

// asVector and goReg fold the whole B0..Q31 range, including both ends.
func TestSVEScalarFileNames(t *testing.T) {
	for _, tt := range []struct {
		r    Reg
		vec  Reg
		goNm string
	}{
		{B0, V0, "V0"}, {H5, V5, "V5"}, {S9, V9, "V9"}, {D17, V17, "V17"}, {Q31, V31, "V31"},
		{Z31, Z31, "Z31"}, {P15, P15, "P15"},
	} {
		if got := asVector(tt.r); got != tt.vec {
			t.Errorf("asVector(%v) = %v, want %v", tt.r, got, tt.vec)
		}
		if got := goReg(tt.r); got != tt.goNm {
			t.Errorf("goReg(%v) = %q, want %q", tt.r, got, tt.goNm)
		}
	}
}

func TestSVESymbol(t *testing.T) {
	useSVETables(t, sveTables{syms: []string{"B", "H", "S", "D"}})
	pinned := &sveDesc{fixed: 2}
	if s, ok := pinned.symbol(0xffffffff); s != "S" || !ok {
		t.Errorf("pinned symbol = %q, %v, want S", s, ok)
	}
	// Don't-care bits are zero in the mask, so the row for 0b1x matches 0b10
	// and 0b11, and a value that matches no row is a decline.
	tab := &sveDesc{fields: []sveField{{4, 2}}, fixed: -1, table: []sveRow{
		{0x3, 0x0, 0}, {0x2, 0x2, 3},
	}}
	for _, tt := range []struct {
		field uint32
		want  string
		ok    bool
	}{{0, "B", true}, {1, "", false}, {2, "D", true}, {3, "D", true}} {
		if s, ok := tab.symbol(tt.field << 4); s != tt.want || ok != tt.ok {
			t.Errorf("symbol(%d) = %q, %v, want %q, %v", tt.field, s, ok, tt.want, tt.ok)
		}
	}
}

func TestSVEGoArrangement(t *testing.T) {
	for _, tt := range []struct {
		a    Arrangement
		want string
	}{
		{ArrangementB, ".B"}, {ArrangementQ, ".Q"}, {ArrangementM, ".M"}, {ArrangementZ, ".Z"},
		{Arrangement8B, ".B8"}, {Arrangement16B, ".B16"}, {Arrangement4H, ".H4"}, {Arrangement2D, ".D2"},
		{Arrangement1Q, ".Q1"}, {0, ""},
	} {
		if got := goArrangement(tt.a); got != tt.want {
			t.Errorf("goArrangement(%v) = %q, want %q", tt.a, got, tt.want)
		}
	}
}

func TestSVEFPImm(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"0.0", "$(0.0)"}, {"0.5", "$(0.5)"}, {"1.0", "$(1.0)"}, {"2.0", "$(2.0)"},
	} {
		arg, ok := sveFPImm(tt.in)
		if !ok {
			t.Errorf("sveFPImm(%q) failed", tt.in)
			continue
		}
		if got := sveGoArg(arg); got != tt.want {
			t.Errorf("sveFPImm(%q) in Go syntax = %q, want %q", tt.in, got, tt.want)
		}
	}
	if _, ok := sveFPImm("3.0"); ok {
		t.Errorf("sveFPImm(3.0) succeeded")
	}
}

func TestSVESymbolSpellings(t *testing.T) {
	for _, tt := range []struct {
		s    string
		want Arrangement
	}{
		{"B", ArrangementB}, {"H", ArrangementH}, {"S", ArrangementS}, {"D", ArrangementD},
		{"Q", ArrangementQ}, {"M", ArrangementM}, {"Z", ArrangementZ},
		{"16B", Arrangement16B}, {"8H", Arrangement8H}, {"4S", Arrangement4S}, {"2D", Arrangement2D},
	} {
		if got, ok := sveArrangement(tt.s); !ok || got != tt.want {
			t.Errorf("sveArrangement(%q) = %v, %v, want %v", tt.s, got, ok, tt.want)
		}
	}
	if _, ok := sveArrangement("8B"); ok {
		t.Errorf("sveArrangement(8B) succeeded")
	}
	for _, tt := range []struct {
		s    string
		want ExtShift
	}{{"LSL", lsl}, {"UXTW", uxtw}, {"SXTW", sxtw}, {"[absent]", 0}} {
		if got, ok := sveExtShift(tt.s); !ok || got != tt.want {
			t.Errorf("sveExtShift(%q) = %v, %v, want %v", tt.s, got, ok, tt.want)
		}
	}
	if _, ok := sveExtShift("SXTX"); ok {
		t.Errorf("sveExtShift(SXTX) succeeded")
	}
	// The element size is read off the letter and the lane count in front is ignored.
	for _, tt := range []struct {
		s    string
		want int64
	}{{"B", 8}, {"16B", 8}, {"H", 16}, {"4H", 16}, {"S", 32}, {"D", 64}, {"1D", 64}, {"Q", 128}, {"M", 0}, {"", 0}} {
		if got := arngESize(tt.s); got != tt.want {
			t.Errorf("arngESize(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

// TestSVEGate checks the generated gate against the forms it gates. For every
// value of bits 31:25 the bit is set exactly when some form can match it, so the
// gate removes work and never a decoding.
func TestSVEGate(t *testing.T) {
	if len(sveForms) < 1000 {
		t.Skip("generated SVE tables are not installed")
	}
	lo, hi := sveGateOf(sveForms)
	if lo != sveGateLo || hi != sveGateHi {
		t.Errorf("gate = %#016x %#016x, want %#016x %#016x from the forms", sveGateLo, sveGateHi, lo, hi)
	}
}

// TestSVEGateKeepsEveryDecoding compares sveDecode, which has the gate, with
// sveScan, which does not, on the words that matter: the pinned and edge-case
// words, two words for each value of bits 31:25 (every other bit set, and
// clear), and 300,000 random words.
func TestSVEGateKeepsEveryDecoding(t *testing.T) {
	if len(sveForms) < 1000 {
		t.Skip("generated SVE tables are not installed")
	}
	var words []uint32
	for _, c := range loadSVECases(t, "plan9") {
		words = append(words, binary.LittleEndian.Uint32(c.enc))
	}
	known := len(words)
	for v := uint32(0); v < 128; v++ {
		words = append(words, v<<25, v<<25|0x1ffffff)
	}
	r := rand.New(rand.NewSource(98))
	for i := 0; i < 300000; i++ {
		words = append(words, r.Uint32())
	}
	var decoded int
	for _, w := range words {
		got, ok := sveDecode(w)
		want, wantOK := sveScan(w)
		if ok != wantOK || ok && got.String() != want.String() {
			t.Fatalf("%#08x: gated %v, %v; ungated %v, %v", w, got, ok, want, wantOK)
		}
		if ok {
			decoded++
		}
	}
	if decoded < known {
		t.Errorf("only %d of %d words decode; the pinned and edge-case words alone are %d", decoded, len(words), known)
	}
}

// sveOrderClash returns a pair of forms, i before j, that both match some word
// although i is not more specific than j: its mask has no more bits set.
func sveOrderClash(forms []sveForm) (i, j int, found bool) {
	for i := range forms {
		for j := i + 1; j < len(forms); j++ {
			fi, fj := &forms[i], &forms[j]
			if (fi.value^fj.value)&fi.mask&fj.mask == 0 && bits.OnesCount32(fi.mask) <= bits.OnesCount32(fj.mask) {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

// TestSVEFormOrder checks that sveScan tries the more specific of any two forms
// that match the same word first. The generator sorts the forms by the number of
// mask bits and leaves a tie in the order of the XML, so two such forms with as
// many bits would decode by an accident of that order.
func TestSVEFormOrder(t *testing.T) {
	if len(sveForms) < 1000 {
		t.Skip("generated SVE tables are not installed")
	}
	if i, j, ok := sveOrderClash(sveForms); ok {
		fi, fj := sveForms[i], sveForms[j]
		t.Errorf("forms %d (%s, %#08x/%#08x) and %d (%s, %#08x/%#08x) match a common word, and the first is not more specific",
			i, sveOpARM[fi.op], fi.value, fi.mask, j, sveOpARM[fj.op], fj.value, fj.mask)
	}
	for k, tc := range []struct {
		forms []sveForm
		clash bool
	}{
		{[]sveForm{{mask: 0xffff0000, value: 0x05200000}, {mask: 0xff00ff00, value: 0x05000000}}, true},  // a tie
		{[]sveForm{{mask: 0xff000000, value: 0x05000000}, {mask: 0xffff0000, value: 0x05200000}}, true},  // out of order
		{[]sveForm{{mask: 0xffff0000, value: 0x05200000}, {mask: 0xffff0000, value: 0x05210000}}, false}, // no common word
		{[]sveForm{{mask: 0xffff0000, value: 0x05200000}, {mask: 0xff000000, value: 0x05000000}}, false},
	} {
		if _, _, got := sveOrderClash(tc.forms); got != tc.clash {
			t.Errorf("case %d: sveOrderClash found a clash = %v, want %v", k, got, tc.clash)
		}
	}
}
