// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package riscv64asm

import (
	"encoding/binary"
	"fmt"
)

type instArgs [6]instArg

// An instFormat describes the format of an instruction encoding.
type instFormat struct {
	mask  uint32
	value uint32
	op    Op
	// args describe how to decode the instruction arguments.
	// args is stored as a fixed-size array.
	// if there are fewer than len(args) arguments, args[i] == 0 marks
	// the end of the argument list.
	args instArgs
}

var (
	errShort   = fmt.Errorf("truncated instruction")
	errUnknown = fmt.Errorf("unknown instruction")
)

var decoderCover []bool

func init() {
	decoderCover = make([]bool, len(instFormats))
}

// Decode decodes the 4 bytes in src as a single instruction.
// TODO: RVC support.
func Decode(src []byte) (inst Inst, err error) {
	if len(src) < 4 {
		return Inst{}, errShort
	}

	x := binary.LittleEndian.Uint32(src)

Search:
	for i, f := range instFormats {
		if (x & f.mask) != f.value {
			continue
		}

		// Decode args.
		var args Args
		for j, aop := range f.args {
			if aop == 0 {
				break
			}
			arg := decodeArg(aop, x, i)
			if arg == nil {
				// Cannot decode argument.
				continue Search
			}
			args[j] = arg
		}

		decoderCover[i] = true
		inst = Inst{
			Op:   f.op,
			Args: args,
			Enc:  x,
		}
		return inst, nil
	}
	return Inst{}, errUnknown
}

// decodeArg decodes the arg described by aop from the instruction bits x.
// It returns nil if x cannot be decoded according to aop.
func decodeArg(aop instArg, x uint32, index int) Arg {
	switch aop {
	case arg_rd:
		return X0 + Reg((x>>7)&((1<<5)-1))

	case arg_rs1:
		return X0 + Reg((x>>15)&((1<<5)-1))

	case arg_rs2:
		return X0 + Reg((x>>20)&((1<<5)-1))

	case arg_rs3:
		return X0 + Reg((x>>27)&((1<<5)-1))

	case arg_fd:
		return F0 + Reg((x>>7)&((1<<5)-1))

	case arg_fs1:
		return F0 + Reg((x>>15)&((1<<5)-1))

	case arg_fs2:
		return F0 + Reg((x>>20)&((1<<5)-1))

	case arg_fs3:
		return F0 + Reg((x>>27)&((1<<5)-1))

	case arg_rs1_amo:
		return AmoReg{X0 + Reg((x>>15)&((1<<5)-1))}

	case arg_rs1_mem:
		var tmp uint32
		tmp = x >> 20
		return RegOffset{X0 + Reg((x>>15)&((1<<5)-1)), Simm{tmp, true, 12}}

	case arg_rs1_store:
		var tmp uint32
		tmp = (x<<20)>>27 |
			(x>>25)<<5
		return RegOffset{X0 + Reg((x>>15)&((1<<5)-1)), Simm{tmp, true, 12}}

	case arg_pred:
		var tmp uint32
		tmp = x << 4 >> 28
		return MemOrder(uint8(tmp))

	case arg_succ:
		var tmp uint32
		tmp = x << 8 >> 28
		return MemOrder(uint8(tmp))

	case arg_csr:
		var tmp uint32
		tmp = x >> 20
		return Csr(tmp)

	case arg_zimm:
		var tmp uint32
		tmp = x << 12 >> 27
		return Uimm{tmp, true}

	case arg_shamt5:
		var tmp uint32
		tmp = x << 7 >> 27
		return Uimm{tmp, false}

	case arg_shamt6:
		var tmp uint32
		tmp = x << 6 >> 26
		return Uimm{tmp, false}

	case arg_imm12:
		var tmp uint32
		tmp = x >> 20
		return Simm{tmp, true, 12}

	case arg_imm20:
		var tmp uint32
		tmp = x >> 12
		return Uimm{tmp, false}

	case arg_jimm20:
		var tmp uint32
		tmp = (x>>31)<<20 |
			(x<<1)>>22<<1 |
			(x<<11)>>31<<11 |
			(x<<12)>>24<<12
		return Simm{tmp, true, 21}

	case arg_simm12:
		var tmp uint32
		tmp = (x<<20)>>27 |
			(x>>25)<<5
		return Simm{tmp, true, 12}

	case arg_bimm12:
		var tmp uint32
		tmp = (x<<20)>>28<<1 |
			(x<<1)>>26<<5 |
			(x<<24)>>31<<11 |
			(x>>31)<<12
		return Simm{tmp, true, 13}

	default:
		return nil
	}
}
