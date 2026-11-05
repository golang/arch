// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package arm64asm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file compares the SVE decoder with two external disassemblers, GNU
// objdump and llvm-mc, using the machinery of ext_test.go. These differential
// tests are skipped unless these variables name the tools, and when a tool is
// too old to know every pinned form: GNU objdump 2.41 does not know SVE2.1 and
// LLVM 15 rejects most of it.
//
//	ARM64ASM_OBJDUMP   a GNU objdump for aarch64, 2.45 or newer
//	ARM64ASM_LLVMMC    llvm-mc, 22 or newer
//
// There are two sets of words. The pins are the forms of svegnucases.txt, on
// which both tools agree with each other, so any difference from the decoder is
// the decoder's. The mutants are every pin with one of its low 25 bits
// flipped, which stay in the SVE encoding space; they reach operand values,
// reserved encodings and unallocated forms that no pin does, and the tools
// disagree with the decoder and with each other there for reasons that
// allowedMismatchSVE has to name.

const (
	minObjdumpMajor, minObjdumpMinor = 2, 45
	minLLVM                          = 22
)

func sveObjdump(t *testing.T) string {
	t.Helper()
	path := os.Getenv("ARM64ASM_OBJDUMP")
	if path == "" {
		t.Skip("ARM64ASM_OBJDUMP is not set")
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		t.Skipf("cannot run %s: %v", path, err)
	}
	m := regexp.MustCompile(`(?m)^GNU objdump .* (\d+)\.(\d+)`).FindSubmatch(out)
	if m == nil {
		t.Skipf("%s is not GNU objdump", path)
	}
	major, _ := strconv.Atoi(string(m[1]))
	minor, _ := strconv.Atoi(string(m[2]))
	if major < minObjdumpMajor || major == minObjdumpMajor && minor < minObjdumpMinor {
		t.Skipf("%s is binutils %s.%s; the SVE pins need %d.%d", path, m[1], m[2], minObjdumpMajor, minObjdumpMinor)
	}
	return path
}

func sveLLVMMC(t *testing.T) string {
	t.Helper()
	path := os.Getenv("ARM64ASM_LLVMMC")
	if path == "" {
		t.Skip("ARM64ASM_LLVMMC is not set")
	}
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		t.Skipf("cannot run %s: %v", path, err)
	}
	m := regexp.MustCompile(`LLVM version (\d+)`).FindSubmatch(out)
	if m == nil {
		t.Skipf("%s is not llvm-mc", path)
	}
	if v, _ := strconv.Atoi(string(m[1])); v < minLLVM {
		t.Skipf("%s is LLVM %s; the SVE pins need %d", path, m[1], minLLVM)
	}
	return path
}

// llvmmc is an extdis function (see testExtDis) for llvm-mc.
func llvmmc(path string) func(*ExtDis) error {
	return func(ext *ExtDis) error {
		buf := make([]byte, ext.Size)
		if _, err := ext.File.ReadAt(buf, start); err != nil {
			return err
		}
		var in bytes.Buffer
		for i := 0; i < len(buf); i += 4 {
			fmt.Fprintf(&in, "0x%02x,0x%02x,0x%02x,0x%02x\n", buf[i], buf[i+1], buf[i+2], buf[i+3])
		}
		cmd := exec.Command(path, "--disassemble", "-triple=aarch64", "-mattr=+all", "-M", "no-aliases")
		var stdout, stderr bytes.Buffer
		cmd.Stdin, cmd.Stdout, cmd.Stderr = &in, &stdout, &stderr
		// llvm-mc reports an invalid encoding as a warning on the line it read.
		if err := cmd.Run(); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				return err
			}
		}
		invalid := map[int]bool{}
		for _, m := range regexp.MustCompile(`<stdin>:(\d+):\d+: warning: invalid instruction encoding`).FindAllStringSubmatch(stderr.String(), -1) {
			n, _ := strconv.Atoi(m[1])
			invalid[n] = true
		}
		var lines []string
		for _, l := range strings.Split(stdout.String(), "\n") {
			if strings.HasPrefix(l, "\t") && !strings.HasPrefix(strings.TrimSpace(l), ".") {
				lines = append(lines, l)
			}
		}
		for i := 0; i < len(buf)/4; i++ {
			d := ExtInst{addr: start + uint64(4*i), nenc: 4}
			copy(d.enc[:], buf[4*i:])
			if invalid[i+1] {
				d.text = "error: unknown instruction"
			} else {
				if len(lines) == 0 {
					return fmt.Errorf("llvm-mc printed too few lines")
				}
				text := lines[0]
				lines = lines[1:]
				if j := strings.Index(text, "//"); j >= 0 {
					text = text[:j]
				}
				d.text = string(fixSpace([]byte(text)))
			}
			ext.Dec <- d
		}
		if len(lines) != 0 {
			return fmt.Errorf("llvm-mc printed %d extra lines", len(lines))
		}
		return nil
	}
}

// sveCounts is what the SVE differential tests add to testExtDis's own report.
// testExtDis only compares words that both sides decode, which hides the two
// outcomes that a decoder is judged by.
type sveCounts struct {
	both     int // the tool and the decoder both decode the word
	gap      int // the tool decodes it and the decoder declines
	notSVE   int // the tool decodes it as something that is not SVE, such as SME
	overAcc  int // the decoder decodes it and the tool declines
	neither  int
	overList []string
}

// counting wraps an extdis function so that every word it reports is classified.
func (c *sveCounts) counting(extdis func(*ExtDis) error) func(*ExtDis) error {
	return func(ext *ExtDis) error {
		// The words as written. The objdump parser leaves the bytes of a word it
		// calls undefined in the order objdump printed them, so the bytes in
		// the report cannot be used to decode here.
		words := make([]byte, ext.Size)
		if _, err := ext.File.ReadAt(words, start); err != nil {
			return err
		}
		in := &ExtDis{Arch: ext.Arch, Dec: make(chan ExtInst), File: ext.File, Size: ext.Size}
		errc := make(chan error, 1)
		go func() {
			err := extdis(in)
			close(in.Dec)
			errc <- err
		}()
		for d := range in.Dec {
			i := int(d.addr - start)
			_, err := Decode(words[i : i+4])
			ours, theirs := err == nil, !strings.Contains(d.text, "unknown")
			switch {
			case ours && theirs:
				c.both++
			case theirs && !sveWord([4]byte(words[i:i+4]), d.text):
				c.notSVE++
			case theirs:
				c.gap++
			case ours:
				c.overAcc++
				if len(c.overList) < 10 {
					c.overList = append(c.overList, fmt.Sprintf("%x", words[i:i+4]))
				}
			default:
				c.neither++
			}
			ext.Dec <- d
		}
		return <-errc
	}
}

// sveWord reports whether a word that a tool decodes as text is SVE: bits 28:25
// equal to 0010, or bits 31:25 equal to 1010000 and the mnemonic one of the
// SVE2.1 multi-vector loads and stores, which sit in the space SME uses for its
// own instructions. A mutant of one of those loads can be an SME instruction
// such as smops, and no SVE decoder will ever decode it.
func sveWord(enc [4]byte, text string) bool {
	w := binary.LittleEndian.Uint32(enc[:])
	if w>>25&15 == 2 {
		return true
	}
	if w>>25 != 0b1010000 {
		return false
	}
	return sveMultiVector.MatchString(text)
}

// sveMultiVector matches the mnemonics of the SVE2.1 multi-vector contiguous
// loads and stores in either syntax.
var sveMultiVector = regexp.MustCompile(`(?i)^[zp]?(ld|st)(nt)?1[bhwd]\b`)

// sveRange matches a register range such as {z8.b-z9.b}.
var sveRange = regexp.MustCompile(`([pz]n?)(\d+)(\.[a-z0-9]+)-[pz]n?(\d+)\.[a-z0-9]+`)

// sveCanon puts text in a form where the spellings the tools and this package
// disagree on, and that mean the same, are equal: no spaces, a register range
// expanded to the list of its registers, and #0.0 written #0.
func sveCanon(s string) string {
	s = strings.Join(strings.Fields(s), "")
	// The objdump parser in objdumpext_test.go rewrites #0.0 to #0.
	s = strings.ReplaceAll(s, "#0.0", "#0")
	return sveRange.ReplaceAllStringFunc(s, func(m string) string {
		p := sveRange.FindStringSubmatch(m)
		lo, _ := strconv.Atoi(p[2])
		hi, _ := strconv.Atoi(p[4])
		var regs []string
		for r := lo; ; r = (r + 1) % 32 {
			regs = append(regs, fmt.Sprintf("%s%d%s", p[1], r, p[3]))
			if r == hi || len(regs) > 4 {
				break
			}
		}
		return strings.Join(regs, ",")
	})
}

// allowedMismatchSVE reports whether the mismatch between text, this package's
// disassembly, and dec, the tool's, is expected. It is deliberately not
// allowedMismatchObjdump: that list excuses any text beginning with mov, orr,
// hint and a dozen other words, which are all SVE mnemonics too, and would turn
// these tests off for them.
func allowedMismatchSVE(text string, inst *Inst, dec ExtInst) bool {
	if strings.HasPrefix(text, "error:") {
		// The decoder declined a word the tool decodes. That is the gap, and it
		// is allowed until the decoder is complete.
		if sveWord(dec.enc, dec.text) {
			return !sveDecoderComplete
		}
		// An SME instruction that shares the space, which this package does
		// not decode and is not asked to.
		return binary.LittleEndian.Uint32(dec.enc[:])>>25 == 0b1010000
	}
	return sveCanon(text) == sveCanon(dec.text)
}

// svePinWords returns the words of svegnucases.txt, which both tools decode alike.
func svePinWords(t *testing.T) [][]byte {
	var words [][]byte
	for _, c := range loadSVEPins(t, "gnu") {
		words = append(words, c.enc)
	}
	return words
}

// sveMutantWords returns every pinned word with one of its low 25 bits flipped,
// in a stable order and without duplicates or pins. Bits 31:25 select the
// encoding group and are left alone, so the words stay in the SVE space.
func sveMutantWords(t *testing.T) [][]byte {
	pins := map[uint32]bool{}
	var order []uint32
	for _, c := range loadSVEPins(t, "plan9") {
		w := binary.LittleEndian.Uint32(c.enc)
		pins[w] = true
		order = append(order, w)
	}
	seen := map[uint32]bool{}
	var mutants []uint32
	for _, w := range order {
		for bit := 0; bit < 25; bit++ {
			m := w ^ 1<<uint(bit)
			if !pins[m] && !seen[m] {
				seen[m] = true
				mutants = append(mutants, m)
			}
		}
	}
	sort.Slice(mutants, func(i, j int) bool { return mutants[i] < mutants[j] })
	words := make([][]byte, len(mutants))
	for i, m := range mutants {
		words[i] = binary.LittleEndian.AppendUint32(nil, m)
	}
	return words
}

func testSVEExt(t *testing.T, extdis func(*ExtDis) error, words [][]byte) {
	var c sveCounts
	testExtDis(t, "gnu", ModeARM64, c.counting(extdis), func(try func([]byte)) {
		for _, w := range words {
			try(w)
		}
	}, allowedMismatchSVE)
	t.Logf("%d words: %d decoded by both, %d only by the tool (the gap), %d only by this package (over-acceptance), %d by neither, %d not SVE",
		len(words), c.both, c.gap, c.overAcc, c.neither, c.notSVE)
	if len(c.overList) > 0 {
		t.Logf("over-accepted, for example: %s", strings.Join(c.overList, " "))
	}
	if sveDecoderComplete && c.gap != 0 {
		t.Errorf("the decoder declines %d words the tool decodes", c.gap)
	}
}

func withObjdump(t *testing.T) func(*ExtDis) error {
	path := sveObjdump(t)
	oldPath, oldFlags := objdumpPath, objdumpFlags
	objdumpPath, objdumpFlags = path, []string{"-M", "no-aliases"}
	t.Cleanup(func() { objdumpPath, objdumpFlags = oldPath, oldFlags })
	return objdump
}

func TestObjdumpARM64SVEPins(t *testing.T)    { testSVEExt(t, withObjdump(t), svePinWords(t)) }
func TestObjdumpARM64SVEMutants(t *testing.T) { testSVEExt(t, withObjdump(t), sveMutantWords(t)) }
func TestLLVMMCARM64SVEPins(t *testing.T) {
	testSVEExt(t, llvmmc(sveLLVMMC(t)), svePinWords(t))
}
func TestLLVMMCARM64SVEMutants(t *testing.T) {
	testSVEExt(t, llvmmc(sveLLVMMC(t)), sveMutantWords(t))
}

// runExtDis runs an extdis function over encs and returns what it reports.
func runExtDis(t *testing.T, extdis func(*ExtDis) error, encs [][]byte) []ExtInst {
	t.Helper()
	file, f, size, err := writeInst(func(try func([]byte)) {
		for _, e := range encs {
			try(e)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		f.Close()
		os.Remove(file)
	}()
	ext := &ExtDis{Dec: make(chan ExtInst), Arch: ModeARM64, File: f, Size: size}
	errc := make(chan error, 1)
	go func() {
		err := extdis(ext)
		if err != nil {
			close(ext.Dec)
		}
		errc <- err
	}()
	var out []ExtInst
	for range encs {
		d, ok := <-ext.Dec
		if !ok {
			break
		}
		out = append(out, d)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if len(out) != len(encs) {
		t.Fatalf("tool reported %d of %d words", len(out), len(encs))
	}
	return out
}

// sveFloatImm matches a floating-point immediate, which Imm_fp prints in one
// spelling and the tools print as #1.0.
var sveFloatImm = regexp.MustCompile(`#-?\d+\.\d+$`)

// TestSVEUpdateCases rewrites svegnucases.txt from the pins in sveplan9cases.txt.
// It does nothing unless -update-sve-cases is set.
func TestSVEUpdateCases(t *testing.T) {
	if !*updateSVECases {
		t.Skip("-update-sve-cases not set")
	}
	pins := loadSVEPins(t, "plan9")
	var encs [][]byte
	for _, p := range pins {
		encs = append(encs, p.enc)
	}
	gnu := runExtDis(t, withObjdump(t), encs)
	llvm := runExtDis(t, llvmmc(sveLLVMMC(t)), encs)
	var gnuOut strings.Builder
	gnuOut.WriteString(sveGNUHeader)
	n := 0
	for i := range encs {
		if strings.Contains(gnu[i].text, "unknown") || strings.Contains(llvm[i].text, "unknown") {
			t.Errorf("%x: a tool does not decode a pinned encoding (objdump %q, llvm-mc %q)", encs[i], gnu[i].text, llvm[i].text)
			continue
		}
		if sveCanon(gnu[i].text) != sveCanon(llvm[i].text) || sveFloatImm.MatchString(llvm[i].text) && !strings.HasSuffix(llvm[i].text, "#0.0") {
			continue
		}
		fmt.Fprintf(&gnuOut, "%x|\t%s\n", encs[i], llvm[i].text)
		n++
	}
	if err := os.WriteFile(filepath.Join("testdata", "svegnucases.txt"), []byte(gnuOut.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d of %d pins that both tools print alike", n, len(pins))
}
