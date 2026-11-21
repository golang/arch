// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x86asm

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func testFormattingSymname(addr uint64) (string, uint64) {
	switch addr {
	case 0x424080:
		return "runtime.printint", 0x424080
	case 0x4c8068:
		return "main.A", 0x4c8068
	}
	return "", 0
}

func TestFormattingCMOV(t *testing.T) {
	// The Go mnemonics and opcodes match cmd/internal/obj/x86/asm6.go.
	conditions := []struct {
		opcode byte
		goOps  [3]string // 16-, 32-, and 64-bit forms
		intel  string
		gnu    string
	}{
		{0x40, [3]string{"CMOVWOS", "CMOVLOS", "CMOVQOS"}, "cmovo", "cmovo"},
		{0x41, [3]string{"CMOVWOC", "CMOVLOC", "CMOVQOC"}, "cmovno", "cmovno"},
		{0x42, [3]string{"CMOVWCS", "CMOVLCS", "CMOVQCS"}, "cmovb", "cmovb"},
		{0x43, [3]string{"CMOVWCC", "CMOVLCC", "CMOVQCC"}, "cmovnb", "cmovae"},
		{0x44, [3]string{"CMOVWEQ", "CMOVLEQ", "CMOVQEQ"}, "cmovz", "cmove"},
		{0x45, [3]string{"CMOVWNE", "CMOVLNE", "CMOVQNE"}, "cmovnz", "cmovne"},
		{0x46, [3]string{"CMOVWLS", "CMOVLLS", "CMOVQLS"}, "cmovbe", "cmovbe"},
		{0x47, [3]string{"CMOVWHI", "CMOVLHI", "CMOVQHI"}, "cmovnbe", "cmova"},
		{0x48, [3]string{"CMOVWMI", "CMOVLMI", "CMOVQMI"}, "cmovs", "cmovs"},
		{0x49, [3]string{"CMOVWPL", "CMOVLPL", "CMOVQPL"}, "cmovns", "cmovns"},
		{0x4a, [3]string{"CMOVWPS", "CMOVLPS", "CMOVQPS"}, "cmovp", "cmovp"},
		{0x4b, [3]string{"CMOVWPC", "CMOVLPC", "CMOVQPC"}, "cmovnp", "cmovnp"},
		{0x4c, [3]string{"CMOVWLT", "CMOVLLT", "CMOVQLT"}, "cmovl", "cmovl"},
		{0x4d, [3]string{"CMOVWGE", "CMOVLGE", "CMOVQGE"}, "cmovnl", "cmovge"},
		{0x4e, [3]string{"CMOVWLE", "CMOVLLE", "CMOVQLE"}, "cmovle", "cmovle"},
		{0x4f, [3]string{"CMOVWGT", "CMOVLGT", "CMOVQGT"}, "cmovnle", "cmovg"},
	}
	widths := []struct {
		bits       int
		prefix     string
		src, dst   string
		memorySize string
	}{
		{16, "66", "ax", "dx", "word"},
		{32, "", "eax", "edx", "dword"},
		{64, "48", "rax", "rdx", "qword"},
	}
	for _, mode := range []int{32, 64} {
		for i, width := range widths {
			if mode == 32 && width.bits == 64 {
				continue
			}
			for _, condition := range conditions {
				for _, memory := range []bool{false, true} {
					name := fmt.Sprintf("mode%d/%s/memory%t", mode, condition.goOps[i], memory)
					t.Run(name, func(t *testing.T) {
						modrm := byte(0xd0) // AX to DX
						goArgs := " AX, DX"
						intelArgs := fmt.Sprintf(" %s, %s", width.dst, width.src)
						gnuArgs := fmt.Sprintf(" %%%s,%%%s", width.src, width.dst)
						if memory {
							modrm = 0x10 // (AX) to DX
							base := "eax"
							if mode == 64 {
								base = "rax"
							}
							goArgs = " 0(AX), DX"
							intelArgs = fmt.Sprintf(" %s, %s ptr [%s]", width.dst, width.memorySize, base)
							gnuArgs = fmt.Sprintf(" (%%%s),%%%s", base, width.dst)
						}
						code, _ := hex.DecodeString(fmt.Sprintf("%s0f%02x%02x", width.prefix, condition.opcode, modrm))
						inst, err := Decode(code, mode)
						if err != nil {
							t.Fatalf("Decode(%x, %d): %v", code, mode, err)
						}
						if got, want := GoSyntax(inst, 0, nil), condition.goOps[i]+goArgs; got != want {
							t.Errorf("GoSyntax(%x): got %q, want %q", code, got, want)
						}
						if got, want := IntelSyntax(inst, 0, nil), condition.intel+intelArgs; got != want {
							t.Errorf("IntelSyntax(%x): got %q, want %q", code, got, want)
						}
						if got, want := GNUSyntax(inst, 0, nil), condition.gnu+gnuArgs; got != want {
							t.Errorf("GNUSyntax(%x): got %q, want %q", code, got, want)
						}
					})
				}
			}
		}
	}
}

func TestFormatting(t *testing.T) {
	testCases := []struct {
		PC    uint64
		bytes string

		goSyntax, intelSyntax, gnuSyntax string
	}{
		{0x4816b2, "0f8677010000",
			"JBE 0x48182f",
			"jbe 0x48182f",
			"jbe 0x48182f"},
		{0x45065b, "488b442408",
			"MOVQ 0x8(SP), AX",
			"mov rax, qword ptr [rsp+0x8]",
			"mov 0x8(%rsp),%rax"},
		{0x450678, "488b05e9790700",
			"MOVQ main.A(SB), AX",
			"mov rax, qword ptr [main.A]",
			"mov main.A,%rax"},
		{0x450664, "e8173afdff",
			"CALL runtime.printint(SB)",
			"call runtime.printint",
			"callq runtime.printint"},
		{0x45069b, "488d0575d90100",
			"LEAQ 0x1d975(IP), AX",
			"lea rax, ptr [rip+0x1d975]",
			"lea 0x1d975(%rip),%rax"},
	}

	for _, testCase := range testCases {
		t.Logf("%#x %s %s", testCase.PC, testCase.bytes, testCase.goSyntax)
		bs, _ := hex.DecodeString(testCase.bytes)
		inst, err := Decode(bs, 64)
		if err != nil {
			t.Errorf("decode error %v", err)
		}
		if out := GoSyntax(inst, testCase.PC, testFormattingSymname); out != testCase.goSyntax {
			t.Errorf("GoSyntax: %q", out)
		}
		if out := IntelSyntax(inst, testCase.PC, testFormattingSymname); out != testCase.intelSyntax {
			t.Errorf("IntelSyntax: %q expected: %q", out, testCase.intelSyntax)
		}
		if out := GNUSyntax(inst, testCase.PC, testFormattingSymname); out != testCase.gnuSyntax {
			t.Errorf("GNUSyntax: %q expected: %q", out, testCase.gnuSyntax)
		}
	}
}
