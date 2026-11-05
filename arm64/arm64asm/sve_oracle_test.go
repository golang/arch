// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is the SVE round-trip test. Every SVE encoding that cmd/asm pins in
// its encoder testdata (arm64sveenc.s, one line per instruction form) is
// decoded, and the result is compared with the text the assembler was given. It
// needs no external tool. It fails on a wrong decoding and counts the words that
// do not decode.
//
// The testdata files hold one case per line in the format of gnucases.txt and
// plan9cases.txt: the four bytes of the instruction in memory order, a vertical
// bar, a tab and the text.
//
//	sveplan9cases.txt  the Go assembler line, for all pinned forms. It is
//	                   written by instgen -pins, in the run that writes the
//	                   pins themselves.
//	svegnucases.txt    the GNU syntax, for the forms on which GNU objdump 2.45
//	                   and llvm-mc 22 print the same text, ignoring spaces and
//	                   the spelling of a register pair ({z8.b-z9.b} or
//	                   {z8.b, z9.b}). Each form is therefore confirmed by two
//	                   disassemblers that share no code with each other or with
//	                   this package. The text is llvm-mc's.
//
//	sveedgecases.txt  encodings that no pin has, such as R18 (which the Go
//	                   assembler calls R18_PLATFORM) or a logical immediate
//	                   with a 2-bit element, with both syntaxes on each line.
//	                   The bytes are what cmd/asm assembles the Go line to. It
//	                   is edited by hand.
//
// The forms missing from svegnucases.txt are the ones on which the two tools
// disagree with each other (the spelling of a zero offset, an immediate that
// llvm-mc repeats across the register width, a lane index it adds) and four
// floating-point immediates that both tools print as #1.0 where Imm_fp prints
// its one spelling.
//
// To refresh sveplan9cases.txt, run instgen with -pins naming it. To refresh
// svegnucases.txt from that file and the tools:
//
//	go test -run TestSVEUpdateCases -update-sve-cases
//
// with ARM64ASM_OBJDUMP naming a GNU objdump for aarch64 (2.45 or newer) and
// ARM64ASM_LLVMMC an llvm-mc 22 or newer.

// sveDecoderComplete says whether Decode is expected to handle every pinned SVE
// encoding. While it is false the test reports how many are undecodable and
// passes; it still fails on a wrong decoding, and it fails if every encoding
// decodes, so that nobody forgets to set this. When it is true any undecodable
// encoding is an error. The generated tables decode all of them.
const sveDecoderComplete = true

const sveGNUHeader = `# SVE round-trip testdata, GNU syntax. The pinned encodings of sveplan9cases.txt
# on which GNU objdump 2.45 and llvm-mc 22 (both with no aliases) print the same
# text, ignoring spaces and the spelling of a register pair. The text is
# llvm-mc's. Read by sve_oracle_test.go, which also says how to regenerate it.
`

var updateSVECases = flag.Bool("update-sve-cases", false, "rewrite svegnucases.txt from the pins, using objdump and llvm-mc")

// An sveCase is one line of an SVE testdata file.
type sveCase struct {
	enc  []byte // the instruction, in memory order
	text string // the expected text
	line int
}

// loadSVEFile reads a testdata file of "bytes|<tab>text" lines. A line may have
// more tab-separated columns after the first, which are returned as cols.
func loadSVEFile(t *testing.T, name string) (cases []sveCase, cols [][]string) {
	t.Helper()
	name = filepath.Join("testdata", name)
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		bar := strings.Index(line, "|\t")
		if bar != 8 {
			t.Fatalf("%s:%d: want 8 hex digits, a bar and a tab", name, n)
		}
		enc, err := hex.DecodeString(line[:bar])
		if err != nil {
			t.Fatalf("%s:%d: %v", name, n, err)
		}
		c := strings.Split(line[bar+2:], "\t")
		cases = append(cases, sveCase{enc, strings.TrimSpace(c[0]), n})
		cols = append(cols, c)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s has no cases", name)
	}
	return cases, cols
}

// loadSVEPins reads the pinned forms in one syntax, "plan9" or "gnu".
func loadSVEPins(t *testing.T, syntax string) []sveCase {
	cases, _ := loadSVEFile(t, "sve"+syntax+"cases.txt")
	return cases
}

// loadSVECases reads the pins in one syntax and then the edge cases, which
// carry both syntaxes.
func loadSVECases(t *testing.T, syntax string) []sveCase {
	t.Helper()
	cases := loadSVEPins(t, syntax)
	edges, cols := loadSVEFile(t, "sveedgecases.txt")
	for i, e := range edges {
		if len(cols[i]) != 2 {
			t.Fatalf("sveedgecases.txt:%d: want a Go line and a GNU line", e.line)
		}
		e.text = strings.TrimSpace(cols[i][map[string]int{"plan9": 0, "gnu": 1}[syntax]])
		cases = append(cases, e)
	}
	return cases
}

// sveSpaceless is how the text is compared. Like decode_test.go it ignores
// spaces: MemExtend prints "[x1,x2]" where the tools print "[x1, x2]", and the
// Go assembler's testdata aligns its columns.
func sveSpaceless(s string) string { return strings.Join(strings.Fields(s), "") }

// sveRoundTrip decodes every case in one syntax and returns the cases that
// decoded to something else and the number that did not decode at all.
func sveRoundTrip(t *testing.T, syntax string, cases []sveCase) (wrong []string, undecodable int) {
	t.Helper()
	for _, c := range cases {
		inst, err := Decode(c.enc)
		if err != nil {
			undecodable++
			continue
		}
		var got string
		switch syntax {
		case "gnu":
			got = GNUSyntax(inst)
		case "plan9":
			got = GoSyntax(inst, 0, nil, nil)
		}
		if sveSpaceless(got) != sveSpaceless(c.text) {
			wrong = append(wrong, fmt.Sprintf("%x decoded as %q, want %q", c.enc, got, c.text))
		}
	}
	return wrong, undecodable
}

func testSVEPins(t *testing.T, syntax string) {
	cases := loadSVECases(t, syntax)
	wrong, undecodable := sveRoundTrip(t, syntax, cases)
	for i, w := range wrong {
		if i == 20 {
			t.Errorf("... and %d more", len(wrong)-i)
			break
		}
		t.Error(w)
	}
	t.Logf("%s syntax: %d pinned encodings, %d round-tripped, %d wrong, %d undecodable",
		syntax, len(cases), len(cases)-len(wrong)-undecodable, len(wrong), undecodable)
	switch {
	case sveDecoderComplete && undecodable != 0:
		t.Errorf("%d pinned encodings are undecodable", undecodable)
	case !sveDecoderComplete && undecodable == 0:
		t.Errorf("every pinned encoding decodes: set sveDecoderComplete to true")
	}
}

// TestSVEPinsGoSyntax checks the Go assembler syntax against the lines that
// cmd/asm's own tests pin.
func TestSVEPinsGoSyntax(t *testing.T) { testSVEPins(t, "plan9") }

// TestSVEPinsGNUSyntax checks the GNU syntax against the text on which two
// independent disassemblers agree.
func TestSVEPinsGNUSyntax(t *testing.T) { testSVEPins(t, "gnu") }

// TestSVEPinsDecodeAsSVE checks that no pinned word is decoded as something that
// is not SVE. The stock decoder predates SVE, and a pattern in its table that
// matched an SVE word would shadow the SVE decoder. It must hold with and
// without a decoder, so it looks at the words and not at the text.
func TestSVEPinsDecodeAsSVE(t *testing.T) {
	for _, c := range loadSVECases(t, "plan9") {
		inst, err := Decode(c.enc)
		if err != nil {
			continue
		}
		if !sveWord([4]byte(c.enc), c.text) {
			t.Errorf("%x decoded as %v, which is not in the SVE encoding space", c.enc, inst)
		}
	}
}
