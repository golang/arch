// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

import (
	"strconv"
	"strings"
	"testing"
)

func TestObjdumpARM64TestDecodeGNUSyntaxdata(t *testing.T) {
	testObjdumpARM64(t, testdataCases(t, "gnu"))
}
func TestObjdumpARM64TestDecodeGoSyntaxdata(t *testing.T) {
	testObjdumpARM64(t, testdataCases(t, "plan9"))
}
func TestObjdumpARM64Manual(t *testing.T) { testObjdumpARM64(t, hexCases(t, objdumpManualTests)) }
func TestObjdumpARM64Cond(t *testing.T)   { testObjdumpARM64(t, condCases(t)) }
func TestObjdumpARM64(t *testing.T)       { testObjdumpARM64(t, JSONCases(t)) }

// objdumpManualTests holds test cases that will be run by TestObjdumpARMManual.
// If you are debugging a few cases that turned up in a longer run, it can be useful
// to list them here and then use -run=Manual, particularly with tracing enabled.
// Note that these are byte sequences, so they must be reversed from the usual
// word presentation.
var objdumpManualTests = `
bf2003d5
9f2003d5
7f2003d5
5f2003d5
3f2003d5
1f2003d5
df4d03d5
ff4d03d5
28d91b14
da6cb530
15e5e514
ff4603d5
df4803d5
bf4100d5
9f3f03d5
9f3e03d5
9f3d03d5
9f3b03d5
9f3a03d5
9f3903d5
9f3703d5
9f3603d5
9f3503d5
9f3303d5
9f3203d5
9f3103d5
ff4603d5
df4803d5
bf4100d5
a3681b53
47dc78d3
0500a012
0500e092
0500a052
0500a0d2
cd5a206e
cd5a202e
743d050e
743d0a0e
743d0c0e
743d084e
`

// allowedMismatchObjdump reports whether the mismatch between text and dec
// should be allowed by the test.
func allowedMismatchObjdump(text string, inst *Inst, dec ExtInst) bool {
	// Skip unsupported instructions
	if hasPrefix(dec.text, todo...) {
		return true
	}
	// GNU objdump has incorrect alias conditions for following instructions
	if inst.Enc&0x000003ff == 0x000003ff && hasPrefix(dec.text, "negs") && hasPrefix(text, "cmp") {
		return true
	}
	// GNU objdump "NV" is equal to our "AL"
	if strings.HasSuffix(dec.text, " nv") && strings.HasSuffix(text, " al") {
		return true
	}
	if strings.HasPrefix(dec.text, "b.nv") && strings.HasPrefix(text, "b.al") {
		return true
	}
	// GNU objdump recognizes invalid binaries as following instructions
	if hasPrefix(dec.text, "hint", "mrs", "msr", "bfc", "orr", "mov") {
		return true
	}
	if strings.HasPrefix(text, "hint") {
		return true
	}
	// GNU objdump recognizes reserved valuse as valid ones
	if strings.Contains(text, "unknown instruction") && hasPrefix(dec.text, "fmla", "fmul", "fmulx", "fcvtzs", "fcvtzu", "fmls", "fmov", "scvtf", "ucvtf") {
		return true
	}
	// Some old objdump recognizes ldur*/stur*/prfum as ldr*/str*/prfm
	for k, v := range oldObjdumpMismatch {
		if strings.HasPrefix(dec.text, k) && strings.Replace(dec.text, k, v, 1) == text {
			return true
		}
	}
	// New objdump supports some newer mnemonics than this package. This
	// package should be updated to support the new mnemonics and the sense
	// of this reversed to continue passing with older objdumps but that
	// requires internal ARM tooling.
	if newForm, ok := newMnemonics[text]; ok && newForm == dec.text {
		return true
	}
	slcMismatches := map[string]string{
		"pldslckeep": "#0x06",
		"plislckeep": "#0x0e",
		"pstslckeep": "#0x16",
		"pldslcstrm": "#0x07",
		"plislcstrm": "#0x0f",
		"pstslcstrm": "#0x17",
	}
	decTextNormalized := dec.text
	for k, v := range slcMismatches {
		decTextNormalized = strings.Replace(decTextNormalized, k, v, -1)
	}

	// GNU objdump misses spaces between operands for some instructions (e.g., "ld1 {v10.2s, v11.2s}, [x23],#16")
	if strings.Replace(text, " ", "", -1) == strings.Replace(decTextNormalized, " ", "", -1) {
		return true
	}

	// GNU objdump uses ranges for register lists (e.g., "{v12.8h-v13.8h}" vs "{v12.8h, v13.8h}")
	if strings.Contains(dec.text, "{") && strings.Contains(dec.text, "}") && strings.Contains(text, "{") {
		i1 := strings.Index(text, "{")
		j1 := strings.Index(text, "}")
		i2 := strings.Index(decTextNormalized, "{")
		j2 := strings.Index(decTextNormalized, "}")
		if i1 >= 0 && j1 > i1 && i2 >= 0 && j2 > i2 {
			list1 := strings.Replace(text[i1+1:j1], " ", "", -1)
			list2 := strings.Replace(decTextNormalized[i2+1:j2], " ", "", -1)
			rest1 := strings.Replace(text[:i1], " ", "", -1) + strings.Replace(text[j1+1:], " ", "", -1)
			rest2 := strings.Replace(decTextNormalized[:i2], " ", "", -1) + strings.Replace(decTextNormalized[j2+1:], " ", "", -1)
			if rest1 == rest2 {
				// The register list is the only differing part.
				p1 := strings.Split(list1, ",")                                                    // Our list (e.g., "v31.8b, v0.8b")
				p2 := strings.FieldsFunc(list2, func(r rune) bool { return r == ',' || r == '-' }) // Their list (e.g., "v31.8b-v0.8b")
				// Check if the sequence of registers in the list matches the range.
				if len(p1) > 0 && len(p2) > 0 {
					// parseReg converts a register name like "v31.8b" into its number (31) and arrangement ("8b").
					parseReg := func(s string) (int, string, bool) {
						if len(s) > 1 && (s[0] == 'v') {
							s = s[1:]
						}
						parts := strings.Split(s, ".")
						if len(parts) == 0 {
							return 0, "", false
						}
						num, err := strconv.Atoi(parts[0])
						if err != nil {
							return 0, "", false
						}
						arr := ""
						if len(parts) > 1 {
							arr = parts[1]
						}
						return num, arr, true
					}

					startNum, startArr, ok1 := parseReg(p2[0])
					endNum, _, ok2 := parseReg(p2[len(p2)-1])

					if ok1 && ok2 {
						// Calculate expected count in a ring buffer wrapping modulo 32.
						count := (endNum-startNum+32)%32 + 1

						if len(p1) == count {
							match := true
							for i, regStr := range p1 {
								regNum, regArr, ok := parseReg(regStr)
								if !ok || regNum != (startNum+i)%32 || regArr != startArr {
									match = false
									break
								}
							}
							if match {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// TODO: system instruction.
var todo = strings.Fields(`
	sys
	at
	ic
	hvc
	smc
`)

// Following instructions can't be covered because they are just aliases to another instructions which are always preferred
var Ncover = strings.Fields(`
	sbfm
	asrv
	bfm
	ubfm
	lslv
	lsrv
	rorv
	ins
	dup
`)

// Some old objdump wrongly decodes following instructions and allow their mismatches to avoid false alarm
var oldObjdumpMismatch = map[string]string{
	//oldObjValue	correctValue
	"ldr":   "ldur",
	"ldrb":  "ldurb",
	"ldrh":  "ldurh",
	"ldrsb": "ldursb",
	"ldrsh": "ldursh",
	"ldrsw": "ldursw",
	"str":   "stur",
	"strb":  "sturb",
	"strh":  "sturh",
	"prfm":  "prfum",
}

var newMnemonics = map[string]string{
	"dsb #0x00": "ssbb",
	"dsb #0x04": "pssbb",
}
