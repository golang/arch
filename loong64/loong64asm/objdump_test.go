// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package loong64asm

import (
	"strconv"
	"strings"
	"testing"
)

func TestObjdumpLoong64TestDecodeGNUSyntaxdata(t *testing.T) {
	testObjdumpLoong64(t, testdataCases(t, "gnu"))
}

func TestObjdumpLoong64TestDecodeGoSyntaxdata(t *testing.T) {
	testObjdumpLoong64(t, testdataCases(t, "plan9"))
}

func TestObjdumpLoong64Manual(t *testing.T) {
	testObjdumpLoong64(t, hexCases(t, objdumpManualTests))
}

// objdumpManualTests holds test cases that will be run by TestObjdumpLoong64Manual.
// If you are debugging a few cases that turned up in a longer run, it can be useful
// to list them here and then use -run=Manual, particularly with tracing enabled.
// Note that these are byte sequences, so they must be reversed from the usual
// word presentation.
var objdumpManualTests = `
00007238
00807238
00004003
00100050
ac410028
ac41002a
ac41c028
ac414028
ac41402a
ac418028
ac41802a
ac397838
acb97938
acb97838
ac397938
ac397a38
acb97b38
acb97a38
ac397b38
ac110026
ac110024
ac390038
ac392038
ac390c38
ac390438
ac392438
ac390838
ac392838
ac391600
ac391400
ac391500
ac418003
`

// allowedMismatchObjdump reports whether the mismatch between text and dec
// should be allowed by the test.
func allowedMismatchObjdump(text string, inst *Inst, dec ExtInst) bool {
	// GNU objdump use register, decode use alias of register, so corrected it in here
	var dec_text = strings.Replace(dec.text, " ", ",", -1)
	var decsp []string = strings.Split(dec_text, ",")
	var num int = cap(decsp)
	for i := 0; i < num; i++ {
		dex := strings.Index(decsp[i], "$r")
		fdex := strings.Index(decsp[i], "$f")
		ddex := strings.Index(decsp[i], "(")
		if ddex > 0 {
			// ldptr.w $r12,$r13,16(0x10)
			decsp[i] = decsp[i][0:ddex]
		}
		xdex := strings.Index(decsp[i], "0x")
		// convert registers to registers aliases
		if dex >= 0 {
			reg, _ := strconv.Atoi(decsp[i][dex+2:])
			// r12~r20 $t0~t8
			if reg >= 12 && reg <= 20 {
				decsp[i] = strings.Join([]string{"t", strconv.Itoa(reg - 12)}, "")
			}
			// r4~r11 $a0~a7
			if reg >= 4 && reg <= 11 {
				decsp[i] = strings.Join([]string{"a", strconv.Itoa(reg - 4)}, "")
			}
			// r23~r31 $s0~s8
			if reg >= 23 && reg <= 31 {
				decsp[i] = strings.Join([]string{"s", strconv.Itoa(reg - 23)}, "")
			}
			// r0 zero
			if reg == 0 {
				decsp[i] = strings.Join([]string{"zero"}, "")
			}
			// r1 ra
			if reg == 1 {
				decsp[i] = strings.Join([]string{"ra"}, "")
			}
			// r2 tp
			if reg == 2 {
				decsp[i] = strings.Join([]string{"tp"}, "")
			}
			// r3 sp
			if reg == 3 {
				decsp[i] = strings.Join([]string{"sp"}, "")
			}
			// r21 x
			if reg == 21 {
				decsp[i] = strings.Join([]string{"x"}, "")
			}
			// r22 fp
			if reg == 22 {
				decsp[i] = strings.Join([]string{"fp"}, "")
			}
		}
		// convert hexadecimal to decimal
		if xdex >= 0 {
			parseint, _ := strconv.ParseInt(decsp[i][xdex+2:], 16, 32)
			decsp[i] = strings.Join([]string{strconv.Itoa(int(parseint))}, "")
		}
		// convert floating-point registers to floating-point aliases
		if fdex >= 0 && !strings.Contains(decsp[i], "$fcc") {
			freg, _ := strconv.Atoi(decsp[i][fdex+2:])
			// f0~f7 fa0~fa7
			if freg >= 0 && freg <= 7 {
				decsp[i] = strings.Join([]string{"fa", strconv.Itoa(freg - 0)}, "")
			}
			// f8~f23 ft0~ft15
			if freg >= 8 && freg <= 23 {
				decsp[i] = strings.Join([]string{"ft", strconv.Itoa(freg - 8)}, "")
			}
			// f24~f31 fs0~fs7
			if freg >= 24 && freg <= 31 {
				decsp[i] = strings.Join([]string{"fs", strconv.Itoa(freg - 24)}, "")
			}
		}
	}

	return false
}
