// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata_test

import (
	"fmt"
	"log"
	"strings"

	"golang.org/x/arch/x86/xeddata"
)

// The "testdata/xedpath" directory contains XED metadata files
// that are supposed to be used for Database initialization.

// Note that XED objects in this file are not real,
// instructions they describe are fictional.

// This example shows how to print raw XED objects using Reader.
// Objects are called "raw" because some of their fields may
// require additional transformations like macro (states) expansion.
func ExampleReader() {
	const xedPath = "testdata/xedpath"

	input := strings.NewReader(`
{
ICLASS: VEXADD
EXCEPTIONS: avx-type-zero
CPL: 2000
CATEGORY: AVX-Q
EXTENSION: AVX-Q
ATTRIBUTES: A B C
PATTERN: VV1 0x07 VL128 V66 V0F MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()
OPERANDS: REG0=XMM_R():w:width_dq:fword64 REG1=XMM_N():r:width_dq:fword64 MEM0:r:width_dq:fword64
}

{
ICLASS: COND_MOV_Z
CPL: 210
CATEGORY: MOV_IF_COND_MET
EXTENSION: BASE
ISA_SET: COND_MOV
FLAGS: READONLY [ zf-tst ]

PATTERN: 0x0F 0x4F MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()
OPERANDS: REG0=GPRv_R():cw MEM0:r:width_v
PATTERN: 0x0F 0x4F MOD[0b11] MOD=3 REG[rrr] RM[nnn]
OPERANDS: REG0=GPRv_R():cw REG1=GPRv_B():r
}`)

	objects, err := xeddata.NewReader(input).ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	for _, o := range objects {
		fmt.Printf("%s (%s):\n", o.Opcode(), o.Extension)
		for _, inst := range o.Insts {
			fmt.Printf("\t[%d] %s\n", inst.Index, inst.Operands)
		}
	}

	//Output:
	// VEXADD (AVX-Q):
	// 	[0] REG0=XMM_R():w:width_dq:fword64 REG1=XMM_N():r:width_dq:fword64 MEM0:r:width_dq:fword64
	// COND_MOV_Z (BASE):
	// 	[0] REG0=GPRv_R():cw MEM0:r:width_v
	// 	[1] REG0=GPRv_R():cw REG1=GPRv_B():r
}

// This example shows how to use ExpandStates and its effects.
func ExampleExpandStates() {
	const xedPath = "testdata/xedpath"

	input := strings.NewReader(`
{
ICLASS: VEXADD
CPL: 3
CATEGORY: ?
EXTENSION: ?
ATTRIBUTES: AT_A AT_B

PATTERN: _M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_128 _M_MAP_0F MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()
OPERANDS: REG0=XMM_R():w:width_dq:fword64 REG1=XMM_N():r:width_dq:fword64 MEM0:r:width_dq:fword64

PATTERN: _M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_128 _M_MAP_0F MOD[0b11] MOD=3 REG[rrr] RM[nnn]
OPERANDS: REG0=XMM_R():w:width_dq:fword64 REG1=XMM_N():r:width_dq:fword64 REG2=XMM_B():r:width_dq:fword64

PATTERN: _M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_256 _M_MAP_0F MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()
OPERANDS: REG0=YMM_R():w:qq:fword64 REG1=YMM_N():r:qq:fword64 MEM0:r:qq:fword64

PATTERN: _M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_256 _M_MAP_0F MOD[0b11] MOD=3 REG[rrr] RM[nnn]
OPERANDS: REG0=YMM_R():w:qq:fword64 REG1=YMM_N():r:qq:fword64 REG2=YMM_B():r:qq:fword64
}`)

	objects, err := xeddata.NewReader(input).ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	db, err := xeddata.NewDatabase(xedPath)
	if err != nil {
		log.Fatal(err)
	}

	for _, o := range objects {
		for _, inst := range o.Insts {
			fmt.Printf("old: %q\n", inst.Pattern)
			fmt.Printf("new: %q\n", xeddata.ExpandStates(db, inst.Pattern))
		}
	}

	//Output:
	// old: "_M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_128 _M_MAP_0F MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()"
	// new: "VEXVALID=1 0x58 VEX_PREFIX=1 VL=0 MAP=1 MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()"
	// old: "_M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_128 _M_MAP_0F MOD[0b11] MOD=3 REG[rrr] RM[nnn]"
	// new: "VEXVALID=1 0x58 VEX_PREFIX=1 VL=0 MAP=1 MOD[0b11] MOD=3 REG[rrr] RM[nnn]"
	// old: "_M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_256 _M_MAP_0F MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()"
	// new: "VEXVALID=1 0x58 VEX_PREFIX=1 VL=1 MAP=1 MOD[mm] MOD!=3 REG[rrr] RM[nnn] MODRM()"
	// old: "_M_VV_TRUE 0x58  _M_VEX_P_66 _M_VLEN_256 _M_MAP_0F MOD[0b11] MOD=3 REG[rrr] RM[nnn]"
	// new: "VEXVALID=1 0x58 VEX_PREFIX=1 VL=1 MAP=1 MOD[0b11] MOD=3 REG[rrr] RM[nnn]"
}

// This example shows how to handle Inst "OPERANDS" field.
func ExampleOperand() {
	const xedPath = "testdata/xedpath"

	input := strings.NewReader(`
{
ICLASS: ADD_N_TIMES # Like IMUL
CPL: 3
CATEGORY: BINARY
EXTENSION: BASE
ISA_SET: I86
FLAGS: MUST [ of-mod sf-u zf-u af-u pf-u cf-mod ]

PATTERN: 0xAA MOD[mm] MOD!=3 REG[0b101] RM[nnn] MODRM()
OPERANDS: MEM0:r:width_v REG0=AX:rw:SUPP REG1=DX:w:SUPP
}`)

	objects, err := xeddata.NewReader(input).ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	db, err := xeddata.NewDatabase(xedPath)
	if err != nil {
		log.Fatal(err)
	}

	inst := objects[0].Insts[0] // Single instruction is enough for this example
	for i, rawOperand := range strings.Fields(inst.Operands) {
		operand, err := xeddata.NewOperand(db, rawOperand)
		if err != nil {
			log.Fatalf("parse operand #%d: %+v", i, err)
		}

		visibility := "implicit"
		if operand.IsVisible() {
			visibility = "explicit"
		}
		fmt.Printf("(%s) %s:\n", visibility, rawOperand)

		fmt.Printf("\tname: %q\n", operand.Name)
		if operand.IsVisible() {
			fmt.Printf("\t32/64bit width: %s/%s bytes\n",
				db.WidthSize(operand.Width, xeddata.OpSize32),
				db.WidthSize(operand.Width, xeddata.OpSize64))
		}
	}

	//Output:
	// (explicit) MEM0:r:width_v:
	// 	name: "MEM0"
	// 	32/64bit width: 4/8 bytes
	// (implicit) REG0=AX:rw:SUPP:
	// 	name: "REG0=AX"
	// (implicit) REG1=DX:w:SUPP:
	// 	name: "REG1=DX"
}
