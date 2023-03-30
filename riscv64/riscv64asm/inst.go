// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package riscv64asm

import (
	"fmt"
	"strings"
)

// An Op is a RISC-V opcode.
type Op uint16

// NOTE: The actual Op values are defined in tables.go.
func (op Op) String() string {
	if (op >= Op(len(opstr))) || (opstr[op] == "") {
		return fmt.Sprintf("Op(%d)", int(op))
	}

	return opstr[op]
}

// An Arg is a single instruction argument.
type Arg interface {
	String() string
}

// An Args holds the instruction arguments.
// If an instruction has fewer than 6 arguments,
// the final elements in the array are nil.
type Args [6]Arg

// An Inst is a single instruction.
type Inst struct {
	Op   Op     // Opcode mnemonic.
	Enc  uint32 // Raw encoding bits.
	Args Args   // Instruction arguments, in RISC-V mamual order.
}

func (i Inst) String() string {
	var args []string
	for _, arg := range i.Args {
		if arg == nil {
			break
		}
		args = append(args, arg.String())
	}

	if len(args) == 0 {
		return i.Op.String()
	} else {
		return i.Op.String() + " " + strings.Join(args, ",")
	}
}

// A Reg is a single register.
// The zero value denotes X0, not the absence of a register.
type Reg uint16

const (
	// General-purpose register
	X0 Reg = iota
	X1
	X2
	X3
	X4
	X5
	X6
	X7
	X8
	X9
	X10
	X11
	X12
	X13
	X14
	X15
	X16
	X17
	X18
	X19
	X20
	X21
	X22
	X23
	X24
	X25
	X26
	X27
	X28
	X29
	X30
	X31

	//Float point register
	F0
	F1
	F2
	F3
	F4
	F5
	F6
	F7
	F8
	F9
	F10
	F11
	F12
	F13
	F14
	F15
	F16
	F17
	F18
	F19
	F20
	F21
	F22
	F23
	F24
	F25
	F26
	F27
	F28
	F29
	F30
	F31
)

func (r Reg) String() string {
	switch {
	case r == X0:
		return "zero"

	case r == X1:
		return "ra"

	case r == X2:
		return "sp"

	case r == X3:
		return "gp"

	case r == X4:
		return "tp"

	case (r >= X5) && (r <= X7):
		return fmt.Sprintf("t%d", int(r-X5))

	case r == X8:
		return "fp"

	case r == X9:
		return "s1"

	case (r >= X10) && (r <= X17):
		return fmt.Sprintf("a%d", int(r-X10))

	case (r >= X18) && (r <= X27):
		return fmt.Sprintf("s%d", int(r-X18)+2)

	case (r >= X28) && (r <= X31):
		return fmt.Sprintf("t%d", int(r-X28)+3)

	case (r >= F0) && (r <= F7):
		return fmt.Sprintf("ft%d", int(r-F0))

	case (r >= F8) && (r <= F9):
		return fmt.Sprintf("fs%d", int(r-F8))

	case (r >= F10) && (r <= F17):
		return fmt.Sprintf("fa%d", int(r-F10))

	case (r >= F18) && (r <= F27):
		return fmt.Sprintf("fs%d", int(r-F18)+2)

	case (r >= F28) && (r <= F31):
		return fmt.Sprintf("ft%d", int(r-F28)+8)

	default:
		return fmt.Sprintf("Unknown(%d)", int(r))
	}
}

type Csr uint16

const (
	USTATUS        Csr = 0x0000
	FFLAGS         Csr = 0x0001
	FRM            Csr = 0x0002
	FCSR           Csr = 0x0003
	UIE            Csr = 0x0004
	UTVEC          Csr = 0x0005
	UTVT           Csr = 0x0007
	VSTART         Csr = 0x0008
	VXSAT          Csr = 0x0009
	VXRM           Csr = 0x000a
	VCSR           Csr = 0x000f
	USCRATCH       Csr = 0x0040
	UEPC           Csr = 0x0041
	UCAUSE         Csr = 0x0042
	UTVAL          Csr = 0x0043
	UIP            Csr = 0x0044
	UNXTI          Csr = 0x0045
	UINTSTATUS     Csr = 0x0046
	USCRATCHCSW    Csr = 0x0048
	USCRATCHCSWL   Csr = 0x0049
	SSTATUS        Csr = 0x0100
	SEDELEG        Csr = 0x0102
	SIDELEG        Csr = 0x0103
	SIE            Csr = 0x0104
	STVEC          Csr = 0x0105
	SCOUNTEREN     Csr = 0x0106
	STVT           Csr = 0x0107
	SSCRATCH       Csr = 0x0140
	SEPC           Csr = 0x0141
	SCAUSE         Csr = 0x0142
	STVAL          Csr = 0x0143
	SIP            Csr = 0x0144
	SNXTI          Csr = 0x0145
	SINTSTATUS     Csr = 0x0146
	SSCRATCHCSW    Csr = 0x0148
	SSCRATCHCSWL   Csr = 0x0149
	SATP           Csr = 0x0180
	VSSTATUS       Csr = 0x0200
	VSIE           Csr = 0x0204
	VSTVEC         Csr = 0x0205
	VSSCRATCH      Csr = 0x0240
	VSEPC          Csr = 0x0241
	VSCAUSE        Csr = 0x0242
	VSTVAL         Csr = 0x0243
	VSIP           Csr = 0x0244
	VSATP          Csr = 0x0280
	MSTATUS        Csr = 0x0300
	MISA           Csr = 0x0301
	MEDELEG        Csr = 0x0302
	MIDELEG        Csr = 0x0303
	MIE            Csr = 0x0304
	MTVEC          Csr = 0x0305
	MCOUNTEREN     Csr = 0x0306
	MTVT           Csr = 0x0307
	MSTATUSH       Csr = 0x0310
	MCOUNTINHIBIT  Csr = 0x0320
	MHPMEVENT3     Csr = 0x0323
	MHPMEVENT4     Csr = 0x0324
	MHPMEVENT5     Csr = 0x0325
	MHPMEVENT6     Csr = 0x0326
	MHPMEVENT7     Csr = 0x0327
	MHPMEVENT8     Csr = 0x0328
	MHPMEVENT9     Csr = 0x0329
	MHPMEVENT10    Csr = 0x032a
	MHPMEVENT11    Csr = 0x032b
	MHPMEVENT12    Csr = 0x032c
	MHPMEVENT13    Csr = 0x032d
	MHPMEVENT14    Csr = 0x032e
	MHPMEVENT15    Csr = 0x032f
	MHPMEVENT16    Csr = 0x0330
	MHPMEVENT17    Csr = 0x0331
	MHPMEVENT18    Csr = 0x0332
	MHPMEVENT19    Csr = 0x0333
	MHPMEVENT20    Csr = 0x0334
	MHPMEVENT21    Csr = 0x0335
	MHPMEVENT22    Csr = 0x0336
	MHPMEVENT23    Csr = 0x0337
	MHPMEVENT24    Csr = 0x0338
	MHPMEVENT25    Csr = 0x0339
	MHPMEVENT26    Csr = 0x033a
	MHPMEVENT27    Csr = 0x033b
	MHPMEVENT28    Csr = 0x033c
	MHPMEVENT29    Csr = 0x033d
	MHPMEVENT30    Csr = 0x033e
	MHPMEVENT31    Csr = 0x033f
	MSCRATCH       Csr = 0x0340
	MEPC           Csr = 0x0341
	MCAUSE         Csr = 0x0342
	MTVAL          Csr = 0x0343
	MIP            Csr = 0x0344
	MNXTI          Csr = 0x0345
	MINTSTATUS     Csr = 0x0346
	MSCRATCHCSW    Csr = 0x0348
	MSCRATCHCSWL   Csr = 0x0349
	MTINST         Csr = 0x034a
	MTVAL2         Csr = 0x034b
	PMPCFG0        Csr = 0x03a0
	PMPCFG1        Csr = 0x03a1
	PMPCFG2        Csr = 0x03a2
	PMPCFG3        Csr = 0x03a3
	PMPADDR0       Csr = 0x03b0
	PMPADDR1       Csr = 0x03b1
	PMPADDR2       Csr = 0x03b2
	PMPADDR3       Csr = 0x03b3
	PMPADDR4       Csr = 0x03b4
	PMPADDR5       Csr = 0x03b5
	PMPADDR6       Csr = 0x03b6
	PMPADDR7       Csr = 0x03b7
	PMPADDR8       Csr = 0x03b8
	PMPADDR9       Csr = 0x03b9
	PMPADDR10      Csr = 0x03ba
	PMPADDR11      Csr = 0x03bb
	PMPADDR12      Csr = 0x03bc
	PMPADDR13      Csr = 0x03bd
	PMPADDR14      Csr = 0x03be
	PMPADDR15      Csr = 0x03bf
	HSTATUS        Csr = 0x0600
	HEDELEG        Csr = 0x0602
	HIDELEG        Csr = 0x0603
	HIE            Csr = 0x0604
	HTIMEDELTA     Csr = 0x0605
	HCOUNTEREN     Csr = 0x0606
	HGEIE          Csr = 0x0607
	HTIMEDELTAH    Csr = 0x0615
	HTVAL          Csr = 0x0643
	HIP            Csr = 0x0644
	HVIP           Csr = 0x0645
	HTINST         Csr = 0x064a
	HGATP          Csr = 0x0680
	TSELECT        Csr = 0x07a0
	TDATA1         Csr = 0x07a1
	TDATA2         Csr = 0x07a2
	TDATA3         Csr = 0x07a3
	TINFO          Csr = 0x07a4
	TCONTROL       Csr = 0x07a5
	MCONTEXT       Csr = 0x07a8
	MNOISE         Csr = 0x07a9
	SCONTEXT       Csr = 0x07aa
	DCSR           Csr = 0x07b0
	DPC            Csr = 0x07b1
	DSCRATCH0      Csr = 0x07b2
	DSCRATCH1      Csr = 0x07b3
	MCYCLE         Csr = 0x0b00
	MINSTRET       Csr = 0x0b02
	MHPMCOUNTER3   Csr = 0x0b03
	MHPMCOUNTER4   Csr = 0x0b04
	MHPMCOUNTER5   Csr = 0x0b05
	MHPMCOUNTER6   Csr = 0x0b06
	MHPMCOUNTER7   Csr = 0x0b07
	MHPMCOUNTER8   Csr = 0x0b08
	MHPMCOUNTER9   Csr = 0x0b09
	MHPMCOUNTER10  Csr = 0x0b0a
	MHPMCOUNTER11  Csr = 0x0b0b
	MHPMCOUNTER12  Csr = 0x0b0c
	MHPMCOUNTER13  Csr = 0x0b0d
	MHPMCOUNTER14  Csr = 0x0b0e
	MHPMCOUNTER15  Csr = 0x0b0f
	MHPMCOUNTER16  Csr = 0x0b10
	MHPMCOUNTER17  Csr = 0x0b11
	MHPMCOUNTER18  Csr = 0x0b12
	MHPMCOUNTER19  Csr = 0x0b13
	MHPMCOUNTER20  Csr = 0x0b14
	MHPMCOUNTER21  Csr = 0x0b15
	MHPMCOUNTER22  Csr = 0x0b16
	MHPMCOUNTER23  Csr = 0x0b17
	MHPMCOUNTER24  Csr = 0x0b18
	MHPMCOUNTER25  Csr = 0x0b19
	MHPMCOUNTER26  Csr = 0x0b1a
	MHPMCOUNTER27  Csr = 0x0b1b
	MHPMCOUNTER28  Csr = 0x0b1c
	MHPMCOUNTER29  Csr = 0x0b1d
	MHPMCOUNTER30  Csr = 0x0b1e
	MHPMCOUNTER31  Csr = 0x0b1f
	MCYCLEH        Csr = 0x0b80
	MINSTRETH      Csr = 0x0b82
	MHPMCOUNTER3H  Csr = 0x0b83
	MHPMCOUNTER4H  Csr = 0x0b84
	MHPMCOUNTER5H  Csr = 0x0b85
	MHPMCOUNTER6H  Csr = 0x0b86
	MHPMCOUNTER7H  Csr = 0x0b87
	MHPMCOUNTER8H  Csr = 0x0b88
	MHPMCOUNTER9H  Csr = 0x0b89
	MHPMCOUNTER10H Csr = 0x0b8a
	MHPMCOUNTER11H Csr = 0x0b8b
	MHPMCOUNTER12H Csr = 0x0b8c
	MHPMCOUNTER13H Csr = 0x0b8d
	MHPMCOUNTER14H Csr = 0x0b8e
	MHPMCOUNTER15H Csr = 0x0b8f
	MHPMCOUNTER16H Csr = 0x0b90
	MHPMCOUNTER17H Csr = 0x0b91
	MHPMCOUNTER18H Csr = 0x0b92
	MHPMCOUNTER19H Csr = 0x0b93
	MHPMCOUNTER20H Csr = 0x0b94
	MHPMCOUNTER21H Csr = 0x0b95
	MHPMCOUNTER22H Csr = 0x0b96
	MHPMCOUNTER23H Csr = 0x0b97
	MHPMCOUNTER24H Csr = 0x0b98
	MHPMCOUNTER25H Csr = 0x0b99
	MHPMCOUNTER26H Csr = 0x0b9a
	MHPMCOUNTER27H Csr = 0x0b9b
	MHPMCOUNTER28H Csr = 0x0b9c
	MHPMCOUNTER29H Csr = 0x0b9d
	MHPMCOUNTER30H Csr = 0x0b9e
	MHPMCOUNTER31H Csr = 0x0b9f
	CYCLE          Csr = 0x0c00
	TIME           Csr = 0x0c01
	INSTRET        Csr = 0x0c02
	HPMCOUNTER3    Csr = 0x0c03
	HPMCOUNTER4    Csr = 0x0c04
	HPMCOUNTER5    Csr = 0x0c05
	HPMCOUNTER6    Csr = 0x0c06
	HPMCOUNTER7    Csr = 0x0c07
	HPMCOUNTER8    Csr = 0x0c08
	HPMCOUNTER9    Csr = 0x0c09
	HPMCOUNTER10   Csr = 0x0c0a
	HPMCOUNTER11   Csr = 0x0c0b
	HPMCOUNTER12   Csr = 0x0c0c
	HPMCOUNTER13   Csr = 0x0c0d
	HPMCOUNTER14   Csr = 0x0c0e
	HPMCOUNTER15   Csr = 0x0c0f
	HPMCOUNTER16   Csr = 0x0c10
	HPMCOUNTER17   Csr = 0x0c11
	HPMCOUNTER18   Csr = 0x0c12
	HPMCOUNTER19   Csr = 0x0c13
	HPMCOUNTER20   Csr = 0x0c14
	HPMCOUNTER21   Csr = 0x0c15
	HPMCOUNTER22   Csr = 0x0c16
	HPMCOUNTER23   Csr = 0x0c17
	HPMCOUNTER24   Csr = 0x0c18
	HPMCOUNTER25   Csr = 0x0c19
	HPMCOUNTER26   Csr = 0x0c1a
	HPMCOUNTER27   Csr = 0x0c1b
	HPMCOUNTER28   Csr = 0x0c1c
	HPMCOUNTER29   Csr = 0x0c1d
	HPMCOUNTER30   Csr = 0x0c1e
	HPMCOUNTER31   Csr = 0x0c1f
	VL             Csr = 0x0c20
	VTYPE          Csr = 0x0c21
	VLENB          Csr = 0x0c22
	CYCLEH         Csr = 0x0c80
	TIMEH          Csr = 0x0c81
	INSTRETH       Csr = 0x0c82
	HPMCOUNTER3H   Csr = 0x0c83
	HPMCOUNTER4H   Csr = 0x0c84
	HPMCOUNTER5H   Csr = 0x0c85
	HPMCOUNTER6H   Csr = 0x0c86
	HPMCOUNTER7H   Csr = 0x0c87
	HPMCOUNTER8H   Csr = 0x0c88
	HPMCOUNTER9H   Csr = 0x0c89
	HPMCOUNTER10H  Csr = 0x0c8a
	HPMCOUNTER11H  Csr = 0x0c8b
	HPMCOUNTER12H  Csr = 0x0c8c
	HPMCOUNTER13H  Csr = 0x0c8d
	HPMCOUNTER14H  Csr = 0x0c8e
	HPMCOUNTER15H  Csr = 0x0c8f
	HPMCOUNTER16H  Csr = 0x0c90
	HPMCOUNTER17H  Csr = 0x0c91
	HPMCOUNTER18H  Csr = 0x0c92
	HPMCOUNTER19H  Csr = 0x0c93
	HPMCOUNTER20H  Csr = 0x0c94
	HPMCOUNTER21H  Csr = 0x0c95
	HPMCOUNTER22H  Csr = 0x0c96
	HPMCOUNTER23H  Csr = 0x0c97
	HPMCOUNTER24H  Csr = 0x0c98
	HPMCOUNTER25H  Csr = 0x0c99
	HPMCOUNTER26H  Csr = 0x0c9a
	HPMCOUNTER27H  Csr = 0x0c9b
	HPMCOUNTER28H  Csr = 0x0c9c
	HPMCOUNTER29H  Csr = 0x0c9d
	HPMCOUNTER30H  Csr = 0x0c9e
	HPMCOUNTER31H  Csr = 0x0c9f
	HGEIP          Csr = 0x0e12
	MVENDORID      Csr = 0x0f11
	MARCHID        Csr = 0x0f12
	MIMPID         Csr = 0x0f13
	MHARTID        Csr = 0x0f14
	MENTROPY       Csr = 0x0f15
)

var csrname = [...]string{
	USTATUS:        "USTATUS",
	FFLAGS:         "FFLAGS",
	FRM:            "FRM",
	FCSR:           "FCSR",
	UIE:            "UIE",
	UTVEC:          "UTVEC",
	UTVT:           "UTVT",
	VSTART:         "VSTART",
	VXSAT:          "VXSAT",
	VXRM:           "VXRM",
	VCSR:           "VCSR",
	USCRATCH:       "USCRATCH",
	UEPC:           "UEPC",
	UCAUSE:         "UCAUSE",
	UTVAL:          "UTVAL",
	UIP:            "UIP",
	UNXTI:          "UNXTI",
	UINTSTATUS:     "UINTSTATUS",
	USCRATCHCSW:    "USCRATCHCSW",
	USCRATCHCSWL:   "USCRATCHCSWL",
	SSTATUS:        "SSTATUS",
	SEDELEG:        "SEDELEG",
	SIDELEG:        "SIDELEG",
	SIE:            "SIE",
	STVEC:          "STVEC",
	SCOUNTEREN:     "SCOUNTEREN",
	STVT:           "STVT",
	SSCRATCH:       "SSCRATCH",
	SEPC:           "SEPC",
	SCAUSE:         "SCAUSE",
	STVAL:          "STVAL",
	SIP:            "SIP",
	SNXTI:          "SNXTI",
	SINTSTATUS:     "SINTSTATUS",
	SSCRATCHCSW:    "SSCRATCHCSW",
	SSCRATCHCSWL:   "SSCRATCHCSWL",
	SATP:           "SATP",
	VSSTATUS:       "VSSTATUS",
	VSIE:           "VSIE",
	VSTVEC:         "VSTVEC",
	VSSCRATCH:      "VSSCRATCH",
	VSEPC:          "VSEPC",
	VSCAUSE:        "VSCAUSE",
	VSTVAL:         "VSTVAL",
	VSIP:           "VSIP",
	VSATP:          "VSATP",
	MSTATUS:        "MSTATUS",
	MISA:           "MISA",
	MEDELEG:        "MEDELEG",
	MIDELEG:        "MIDELEG",
	MIE:            "MIE",
	MTVEC:          "MTVEC",
	MCOUNTEREN:     "MCOUNTEREN",
	MTVT:           "MTVT",
	MSTATUSH:       "MSTATUSH",
	MCOUNTINHIBIT:  "MCOUNTINHIBIT",
	MHPMEVENT3:     "MHPMEVENT3",
	MHPMEVENT4:     "MHPMEVENT4",
	MHPMEVENT5:     "MHPMEVENT5",
	MHPMEVENT6:     "MHPMEVENT6",
	MHPMEVENT7:     "MHPMEVENT7",
	MHPMEVENT8:     "MHPMEVENT8",
	MHPMEVENT9:     "MHPMEVENT9",
	MHPMEVENT10:    "MHPMEVENT10",
	MHPMEVENT11:    "MHPMEVENT11",
	MHPMEVENT12:    "MHPMEVENT12",
	MHPMEVENT13:    "MHPMEVENT13",
	MHPMEVENT14:    "MHPMEVENT14",
	MHPMEVENT15:    "MHPMEVENT15",
	MHPMEVENT16:    "MHPMEVENT16",
	MHPMEVENT17:    "MHPMEVENT17",
	MHPMEVENT18:    "MHPMEVENT18",
	MHPMEVENT19:    "MHPMEVENT19",
	MHPMEVENT20:    "MHPMEVENT20",
	MHPMEVENT21:    "MHPMEVENT21",
	MHPMEVENT22:    "MHPMEVENT22",
	MHPMEVENT23:    "MHPMEVENT23",
	MHPMEVENT24:    "MHPMEVENT24",
	MHPMEVENT25:    "MHPMEVENT25",
	MHPMEVENT26:    "MHPMEVENT26",
	MHPMEVENT27:    "MHPMEVENT27",
	MHPMEVENT28:    "MHPMEVENT28",
	MHPMEVENT29:    "MHPMEVENT29",
	MHPMEVENT30:    "MHPMEVENT30",
	MHPMEVENT31:    "MHPMEVENT31",
	MSCRATCH:       "MSCRATCH",
	MEPC:           "MEPC",
	MCAUSE:         "MCAUSE",
	MTVAL:          "MTVAL",
	MIP:            "MIP",
	MNXTI:          "MNXTI",
	MINTSTATUS:     "MINTSTATUS",
	MSCRATCHCSW:    "MSCRATCHCSW",
	MSCRATCHCSWL:   "MSCRATCHCSWL",
	MTINST:         "MTINST",
	MTVAL2:         "MTVAL2",
	PMPCFG0:        "PMPCFG0",
	PMPCFG1:        "PMPCFG1",
	PMPCFG2:        "PMPCFG2",
	PMPCFG3:        "PMPCFG3",
	PMPADDR0:       "PMPADDR0",
	PMPADDR1:       "PMPADDR1",
	PMPADDR2:       "PMPADDR2",
	PMPADDR3:       "PMPADDR3",
	PMPADDR4:       "PMPADDR4",
	PMPADDR5:       "PMPADDR5",
	PMPADDR6:       "PMPADDR6",
	PMPADDR7:       "PMPADDR7",
	PMPADDR8:       "PMPADDR8",
	PMPADDR9:       "PMPADDR9",
	PMPADDR10:      "PMPADDR10",
	PMPADDR11:      "PMPADDR11",
	PMPADDR12:      "PMPADDR12",
	PMPADDR13:      "PMPADDR13",
	PMPADDR14:      "PMPADDR14",
	PMPADDR15:      "PMPADDR15",
	HSTATUS:        "HSTATUS",
	HEDELEG:        "HEDELEG",
	HIDELEG:        "HIDELEG",
	HIE:            "HIE",
	HTIMEDELTA:     "HTIMEDELTA",
	HCOUNTEREN:     "HCOUNTEREN",
	HGEIE:          "HGEIE",
	HTIMEDELTAH:    "HTIMEDELTAH",
	HTVAL:          "HTVAL",
	HIP:            "HIP",
	HVIP:           "HVIP",
	HTINST:         "HTINST",
	HGATP:          "HGATP",
	TSELECT:        "TSELECT",
	TDATA1:         "TDATA1",
	TDATA2:         "TDATA2",
	TDATA3:         "TDATA3",
	TINFO:          "TINFO",
	TCONTROL:       "TCONTROL",
	MCONTEXT:       "MCONTEXT",
	MNOISE:         "MNOISE",
	SCONTEXT:       "SCONTEXT",
	DCSR:           "DCSR",
	DPC:            "DPC",
	DSCRATCH0:      "DSCRATCH0",
	DSCRATCH1:      "DSCRATCH1",
	MCYCLE:         "MCYCLE",
	MINSTRET:       "MINSTRET",
	MHPMCOUNTER3:   "MHPMCOUNTER3",
	MHPMCOUNTER4:   "MHPMCOUNTER4",
	MHPMCOUNTER5:   "MHPMCOUNTER5",
	MHPMCOUNTER6:   "MHPMCOUNTER6",
	MHPMCOUNTER7:   "MHPMCOUNTER7",
	MHPMCOUNTER8:   "MHPMCOUNTER8",
	MHPMCOUNTER9:   "MHPMCOUNTER9",
	MHPMCOUNTER10:  "MHPMCOUNTER10",
	MHPMCOUNTER11:  "MHPMCOUNTER11",
	MHPMCOUNTER12:  "MHPMCOUNTER12",
	MHPMCOUNTER13:  "MHPMCOUNTER13",
	MHPMCOUNTER14:  "MHPMCOUNTER14",
	MHPMCOUNTER15:  "MHPMCOUNTER15",
	MHPMCOUNTER16:  "MHPMCOUNTER16",
	MHPMCOUNTER17:  "MHPMCOUNTER17",
	MHPMCOUNTER18:  "MHPMCOUNTER18",
	MHPMCOUNTER19:  "MHPMCOUNTER19",
	MHPMCOUNTER20:  "MHPMCOUNTER20",
	MHPMCOUNTER21:  "MHPMCOUNTER21",
	MHPMCOUNTER22:  "MHPMCOUNTER22",
	MHPMCOUNTER23:  "MHPMCOUNTER23",
	MHPMCOUNTER24:  "MHPMCOUNTER24",
	MHPMCOUNTER25:  "MHPMCOUNTER25",
	MHPMCOUNTER26:  "MHPMCOUNTER26",
	MHPMCOUNTER27:  "MHPMCOUNTER27",
	MHPMCOUNTER28:  "MHPMCOUNTER28",
	MHPMCOUNTER29:  "MHPMCOUNTER29",
	MHPMCOUNTER30:  "MHPMCOUNTER30",
	MHPMCOUNTER31:  "MHPMCOUNTER31",
	MCYCLEH:        "MCYCLEH",
	MINSTRETH:      "MINSTRETH",
	MHPMCOUNTER3H:  "MHPMCOUNTER3H",
	MHPMCOUNTER4H:  "MHPMCOUNTER4H",
	MHPMCOUNTER5H:  "MHPMCOUNTER5H",
	MHPMCOUNTER6H:  "MHPMCOUNTER6H",
	MHPMCOUNTER7H:  "MHPMCOUNTER7H",
	MHPMCOUNTER8H:  "MHPMCOUNTER8H",
	MHPMCOUNTER9H:  "MHPMCOUNTER9H",
	MHPMCOUNTER10H: "MHPMCOUNTER10H",
	MHPMCOUNTER11H: "MHPMCOUNTER11H",
	MHPMCOUNTER12H: "MHPMCOUNTER12H",
	MHPMCOUNTER13H: "MHPMCOUNTER13H",
	MHPMCOUNTER14H: "MHPMCOUNTER14H",
	MHPMCOUNTER15H: "MHPMCOUNTER15H",
	MHPMCOUNTER16H: "MHPMCOUNTER16H",
	MHPMCOUNTER17H: "MHPMCOUNTER17H",
	MHPMCOUNTER18H: "MHPMCOUNTER18H",
	MHPMCOUNTER19H: "MHPMCOUNTER19H",
	MHPMCOUNTER20H: "MHPMCOUNTER20H",
	MHPMCOUNTER21H: "MHPMCOUNTER21H",
	MHPMCOUNTER22H: "MHPMCOUNTER22H",
	MHPMCOUNTER23H: "MHPMCOUNTER23H",
	MHPMCOUNTER24H: "MHPMCOUNTER24H",
	MHPMCOUNTER25H: "MHPMCOUNTER25H",
	MHPMCOUNTER26H: "MHPMCOUNTER26H",
	MHPMCOUNTER27H: "MHPMCOUNTER27H",
	MHPMCOUNTER28H: "MHPMCOUNTER28H",
	MHPMCOUNTER29H: "MHPMCOUNTER29H",
	MHPMCOUNTER30H: "MHPMCOUNTER30H",
	MHPMCOUNTER31H: "MHPMCOUNTER31H",
	CYCLE:          "CYCLE",
	TIME:           "TIME",
	INSTRET:        "INSTRET",
	HPMCOUNTER3:    "HPMCOUNTER3",
	HPMCOUNTER4:    "HPMCOUNTER4",
	HPMCOUNTER5:    "HPMCOUNTER5",
	HPMCOUNTER6:    "HPMCOUNTER6",
	HPMCOUNTER7:    "HPMCOUNTER7",
	HPMCOUNTER8:    "HPMCOUNTER8",
	HPMCOUNTER9:    "HPMCOUNTER9",
	HPMCOUNTER10:   "HPMCOUNTER10",
	HPMCOUNTER11:   "HPMCOUNTER11",
	HPMCOUNTER12:   "HPMCOUNTER12",
	HPMCOUNTER13:   "HPMCOUNTER13",
	HPMCOUNTER14:   "HPMCOUNTER14",
	HPMCOUNTER15:   "HPMCOUNTER15",
	HPMCOUNTER16:   "HPMCOUNTER16",
	HPMCOUNTER17:   "HPMCOUNTER17",
	HPMCOUNTER18:   "HPMCOUNTER18",
	HPMCOUNTER19:   "HPMCOUNTER19",
	HPMCOUNTER20:   "HPMCOUNTER20",
	HPMCOUNTER21:   "HPMCOUNTER21",
	HPMCOUNTER22:   "HPMCOUNTER22",
	HPMCOUNTER23:   "HPMCOUNTER23",
	HPMCOUNTER24:   "HPMCOUNTER24",
	HPMCOUNTER25:   "HPMCOUNTER25",
	HPMCOUNTER26:   "HPMCOUNTER26",
	HPMCOUNTER27:   "HPMCOUNTER27",
	HPMCOUNTER28:   "HPMCOUNTER28",
	HPMCOUNTER29:   "HPMCOUNTER29",
	HPMCOUNTER30:   "HPMCOUNTER30",
	HPMCOUNTER31:   "HPMCOUNTER31",
	VL:             "VL",
	VTYPE:          "VTYPE",
	VLENB:          "VLENB",
	CYCLEH:         "CYCLEH",
	TIMEH:          "TIMEH",
	INSTRETH:       "INSTRETH",
	HPMCOUNTER3H:   "HPMCOUNTER3H",
	HPMCOUNTER4H:   "HPMCOUNTER4H",
	HPMCOUNTER5H:   "HPMCOUNTER5H",
	HPMCOUNTER6H:   "HPMCOUNTER6H",
	HPMCOUNTER7H:   "HPMCOUNTER7H",
	HPMCOUNTER8H:   "HPMCOUNTER8H",
	HPMCOUNTER9H:   "HPMCOUNTER9H",
	HPMCOUNTER10H:  "HPMCOUNTER10H",
	HPMCOUNTER11H:  "HPMCOUNTER11H",
	HPMCOUNTER12H:  "HPMCOUNTER12H",
	HPMCOUNTER13H:  "HPMCOUNTER13H",
	HPMCOUNTER14H:  "HPMCOUNTER14H",
	HPMCOUNTER15H:  "HPMCOUNTER15H",
	HPMCOUNTER16H:  "HPMCOUNTER16H",
	HPMCOUNTER17H:  "HPMCOUNTER17H",
	HPMCOUNTER18H:  "HPMCOUNTER18H",
	HPMCOUNTER19H:  "HPMCOUNTER19H",
	HPMCOUNTER20H:  "HPMCOUNTER20H",
	HPMCOUNTER21H:  "HPMCOUNTER21H",
	HPMCOUNTER22H:  "HPMCOUNTER22H",
	HPMCOUNTER23H:  "HPMCOUNTER23H",
	HPMCOUNTER24H:  "HPMCOUNTER24H",
	HPMCOUNTER25H:  "HPMCOUNTER25H",
	HPMCOUNTER26H:  "HPMCOUNTER26H",
	HPMCOUNTER27H:  "HPMCOUNTER27H",
	HPMCOUNTER28H:  "HPMCOUNTER28H",
	HPMCOUNTER29H:  "HPMCOUNTER29H",
	HPMCOUNTER30H:  "HPMCOUNTER30H",
	HPMCOUNTER31H:  "HPMCOUNTER31H",
	HGEIP:          "HGEIP",
	MVENDORID:      "MVENDORID",
	MARCHID:        "MARCHID",
	MIMPID:         "MIMPID",
	MHARTID:        "MHARTID",
	MENTROPY:       "MENTROPY",
}

func (c Csr) String() string {
	return csrname[c]
}

type Uimm struct {
	Imm     uint32
	Decimal bool
}

func (ui Uimm) String() string {
	if ui.Decimal == true {
		return fmt.Sprintf("%d", ui.Imm)
	} else {
		return fmt.Sprintf("%#x", ui.Imm)
	}
}

type Simm struct {
	Imm     uint32
	Decimal bool
	Width   uint8
}

func (si Simm) String() string {
	// Sign-extend
	if si.Imm>>uint32(si.Width-1) == 1 {
		var tmp uint32
		tmp = 0xffffffff << si.Width
		si.Imm |= tmp
	}

	if si.Decimal == true {
		return fmt.Sprintf("%d", int32(si.Imm))
	} else {
		return fmt.Sprintf("%#x", int32(si.Imm))
	}
}

// Avoid recursive of String() method.
type AmoReg struct {
	reg Reg
}

func (amoReg AmoReg) String() string {
	return "(" + amoReg.reg.String() + ")"
}

type RegOffset struct {
	reg Reg
	ofs Simm
}

func (regofs RegOffset) String() string {
	return regofs.ofs.String() + "(" + regofs.reg.String() + ")"
}

type MemOrder uint8

func (memOrder MemOrder) String() string {
	var str string
	if memOrder<<7>>7 == 1 {
		str += "i"
	}
	if memOrder>>1<<7>>7 == 1 {
		str += "o"
	}
	if memOrder>>2<<7>>7 == 1 {
		str += "r"
	}
	if memOrder>>3<<7>>7 == 1 {
		str += "w"
	}
	return str
}
