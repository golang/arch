// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package riscv64asm

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

const objdumpPath = "/usr/bin/objdump"

func testObjdumpRISCV64(t *testing.T, generate func(func([]byte))) {
	testObjdumpArch(t, generate)
}

func testObjdumpArch(t *testing.T, generate func(func([]byte))) {
	checkObjdumpRISCV64(t)
	testExtDis(t, "gnu", objdump, generate, allowedMismatchObjdump)
	testExtDis(t, "plan9", objdump, generate, allowedMismatchObjdump)
}

func checkObjdumpRISCV64(t *testing.T) {
	out, err := exec.Command(objdumpPath, "-i").Output()
	if err != nil {
		t.Skipf("cannot run objdump: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "riscv") {
		t.Skip("objdump does not have RISC-V support")
	}
}

func objdump(ext *ExtDis) error {
	// File already written with instructions; add ELF header.
	if err := writeELF64(ext.File, ext.Size); err != nil {
		return err
	}

	b, err := ext.Run(objdumpPath, "-d", "-z", ext.File.Name())
	if err != nil {
		return err
	}

	var (
		nmatch  int
		reading bool
		next    uint64 = start
		addr    uint64
		encbuf  [4]byte
		enc     []byte
		text    string
	)
	flush := func() {
		if addr == next {
			// PC-relative addresses are translated to absolute addresses based on PC by GNU objdump
			// Following logical rewrites the absolute addresses back to PC-relative ones for comparing
			// with our disassembler output which are PC-relative
			if text == "undefined" && len(enc) == 4 {
				text = "error: unknown instruction"
				enc = nil
			}
			if len(enc) == 4 {
				// prints as word but we want to record bytes
				enc[0], enc[3] = enc[3], enc[0]
				enc[1], enc[2] = enc[2], enc[1]
			}
			ext.Dec <- ExtInst{addr, encbuf, len(enc), text}
			encbuf = [4]byte{}
			enc = nil
			next += 4
		}
	}
	var textangle = []byte("<.text>:")
	for {
		line, err := b.ReadSlice('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading objdump output: %v", err)
		}
		if bytes.Contains(line, textangle) {
			reading = true
			continue
		}
		if !reading {
			continue
		}
		if debug {
			os.Stdout.Write(line)
		}
		if enc1 := parseContinuation(line, encbuf[:len(enc)]); enc1 != nil {
			enc = enc1
			continue
		}
		flush()
		nmatch++
		addr, enc, text = parseLine(line, encbuf[:0])
		if addr > next {
			return fmt.Errorf("address out of sync expected <= %#x at %q in:\n%s", next, line, line)
		}
	}
	flush()
	if next != start+uint64(ext.Size) {
		return fmt.Errorf("not enough results found [%d %d]", next, start+ext.Size)
	}
	if err := ext.Wait(); err != nil {
		return fmt.Errorf("exec: %v", err)
	}

	return nil
}

var (
	undefined     = []byte("undefined")
	unpredictable = []byte("unpredictable")
	slashslash    = []byte("//")
)

func parseLine(line []byte, encstart []byte) (addr uint64, enc []byte, text string) {
	ok := false
	oline := line
	i := index(line, ":\t")
	if i < 0 {
		log.Fatalf("cannot parse disassembly: %q", oline)
	}
	x, err := strconv.ParseUint(string(bytes.TrimSpace(line[:i])), 16, 32)
	if err != nil {
		log.Fatalf("cannot parse disassembly: %q", oline)
	}
	addr = uint64(x)
	line = line[i+2:]
	i = bytes.IndexByte(line, '\t')
	if i < 0 {
		log.Fatalf("cannot parse disassembly: %q", oline)
	}
	enc, ok = parseHex(line[:i], encstart)
	if !ok {
		log.Fatalf("cannot parse disassembly: %q", oline)
	}
	line = bytes.TrimSpace(line[i:])
	if bytes.Contains(line, undefined) {
		text = "undefined"
		return
	}
	if false && bytes.Contains(line, unpredictable) {
		text = "unpredictable"
		return
	}
	// Strip trailing comment starting with '#'
	if i := bytes.IndexByte(line, '#'); i >= 0 {
		line = bytes.TrimSpace(line[:i])
	}
	// Strip trailing comment starting with "//"
	if i := bytes.Index(line, slashslash); i >= 0 {
		line = bytes.TrimSpace(line[:i])
	}
	text = string(fixSpace(line))
	return
}

func parseContinuation(line []byte, enc []byte) []byte {
	i := index(line, ":\t")
	if i < 0 {
		return nil
	}
	line = line[i+1:]
	enc, _ = parseHex(line, enc)
	return enc
}

// writeELF64 writes an ELF64 header to the file, describing a text
// segment that starts at start (0x8000) and extends for size bytes.
func writeELF64(f *os.File, size int) error {
	f.Seek(0, io.SeekStart)
	var hdr elf.Header64
	var prog elf.Prog64
	var sect elf.Section64
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, &hdr)
	off1 := buf.Len()
	binary.Write(&buf, binary.LittleEndian, &prog)
	off2 := buf.Len()
	binary.Write(&buf, binary.LittleEndian, &sect)
	off3 := buf.Len()
	buf.Reset()
	data := byte(elf.ELFDATA2LSB)
	hdr = elf.Header64{
		Ident:     [16]byte{0x7F, 'E', 'L', 'F', 2, data, 1},
		Type:      2,
		Machine:   uint16(elf.EM_RISCV),
		Version:   1,
		Entry:     start,
		Phoff:     uint64(off1),
		Shoff:     uint64(off2),
		Flags:     0x5,
		Ehsize:    uint16(off1),
		Phentsize: uint16(off2 - off1),
		Phnum:     1,
		Shentsize: uint16(off3 - off2),
		Shnum:     4,
		Shstrndx:  3,
	}
	binary.Write(&buf, binary.LittleEndian, &hdr)
	prog = elf.Prog64{
		Type:   1,
		Off:    start,
		Vaddr:  start,
		Paddr:  start,
		Filesz: uint64(size),
		Memsz:  uint64(size),
		Flags:  5,
		Align:  start,
	}
	binary.Write(&buf, binary.LittleEndian, &prog)
	binary.Write(&buf, binary.LittleEndian, &sect) // NULL section
	sect = elf.Section64{
		Name:      1,
		Type:      uint32(elf.SHT_PROGBITS),
		Addr:      start,
		Off:       start,
		Size:      uint64(size),
		Flags:     uint64(elf.SHF_ALLOC | elf.SHF_EXECINSTR),
		Addralign: 4,
	}
	binary.Write(&buf, binary.LittleEndian, &sect) // .text
	strtabsize := len("\x00.text\x00.riscv.attributes\x00.shstrtab\x00")
	// RISC-V objdump needs the .riscv.attributes section to identify
	// the RV64G (not include compressed) extensions.
	sect = elf.Section64{
		Name:      uint32(len("\x00.text\x00")),
		Type:      uint32(0x70000003), // SHT_RISCV_ATTRIBUTES
		Addr:      0,
		Off:       uint64(off2 + (off3-off2)*4 + strtabsize),
		Size:      102,
		Addralign: 1,
	}
	binary.Write(&buf, binary.LittleEndian, &sect)
	sect = elf.Section64{
		Name:      uint32(len("\x00.text\x00.riscv.attributes\x00")),
		Type:      uint32(elf.SHT_STRTAB),
		Addr:      0,
		Off:       uint64(off2 + (off3-off2)*4),
		Size:      uint64(strtabsize),
		Addralign: 1,
	}
	binary.Write(&buf, binary.LittleEndian, &sect)
	buf.WriteString("\x00.text\x00.riscv.attributes\x00.shstrtab\x00")
	// Contents of .riscv.attributes section
	// which specify the extension and priv spec version. (1.11)
	buf.WriteString("Ae\x00\x00\x00riscv\x00\x01[\x00\x00\x00\x05rv64i2p1_m2pp_a2p1_f2p2_d2p2_q2p2_zibsr2p0_zifencei2p0_zmmul1p0_zfh1p0_zfhmin1p0\x00\x08\x01\x0a\x0b")
	f.Write(buf.Bytes())
	return nil
}
