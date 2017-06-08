// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package x86asm

import (
	"encoding/hex"
	"testing"
)

func testFormattingSymname(addr uint64) (string, uint64) {
	switch addr {
	case 0x424080:
		return "runtime.printint", 0x424080
	}
	return "", 0
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
			"MOVQ 0x779e9(IP), AX",
			"mov rax, qword ptr [rip+0x779e9]",
			"mov 0x779e9(%rip),%rax"},
		{0x450664, "e8173afdff",
			"CALL runtime.printint(SB)",
			"call 0x424080",
			"callq 0x424080"},
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
		if out := IntelSyntax(inst, testCase.PC); out != testCase.intelSyntax {
			t.Errorf("IntelSyntax: %q expected: %q", out, testCase.intelSyntax)
		}
		if out := GNUSyntax(inst, testCase.PC); out != testCase.gnuSyntax {
			t.Errorf("GNUSyntax: %q expected: %q", out, testCase.gnuSyntax)
		}
	}
}
