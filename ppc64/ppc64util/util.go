// Copyright 2021 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

// Generate interesting test cases from ppc64 objdump via
// go run util.go
//
// This requires powerpc64le-linux-gnu-gcc and powerpc64le-linux-gnu-objdump be in
// the PATH this command is run.
//
// These tools can be acquired from the IBM advance toolchain for amd64 hosts too.

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Generator for branch on spr (bclr, bctar, bcctr)
func emitBSpr(bo, bi, l uint32, out io.Writer) {
	var insn [3]uint32 = [3]uint32{19<<26 | 16<<1, 19<<26 | 528<<1, 19<<26 | 560<<1}
	for bh := uint32(0); bh < 3; bh++ {
		for _, m := range insn {
			m |= bo << 21
			m |= bi << 16
			m |= bh << 11
			m |= l << 0
			fmt.Fprintf(out, "\t.long 0x%08x\n", m)
		}
	}
}

// Generator for bc
func emitBc(bo, bi, l uint32, out io.Writer) {
	for aa := uint32(0); aa < 2; aa++ {
		m := uint32(16 << 26)
		m |= bo << 21
		m |= bi << 16
		m |= l << 0
		m |= aa << 1
		m |= 128
		fmt.Fprintf(out, "\t.long 0x%08x\n", m)
	}
}

// Generator all interesting conditional branch type instructions
func emitBranches(out io.Writer) {
	fmt.Fprintf(out, ".text\n")
	for bo := 0; bo < 0x20; bo++ {
		// objdump behaves strangely on some cases when a z bit is set.
		// Ignore these, they should never show up in correct code.
		if bo&0x15 == 0x1 {
			// skip 0b0.0.z cases where z != 0
			continue
		}
		if bo&0x14 == 0x14 && bo != 14 {
			// skip 0b1z1zz cases where z != 0
			continue
		}
		// skip at == 1 cases.  objdump doesn't handle these well either.
		reserved_at := map[int]bool{5: true, 13: true, 17: true, 19: true}
		if reserved_at[bo] {
			continue
		}
		// only test cr0/cr1 bits. cr2-cr7 cases are basically identical to cr1.
		for bi := 0; bi < 0x8; bi++ {
			for l := 0; l < 2; l++ {
				emitBSpr(uint32(bo), uint32(bi), uint32(l), out)
				emitBc(uint32(bo), uint32(bi), uint32(l), out)
			}
		}
	}
}

// Emit a test file using the generator called name.txt.  This requires
// a GCC toolchain which supports -mcpu=power10.
func genOutput(name, tcPfx string, generator func(io.Writer)) {
	// Generate object code from gcc
	cmd := exec.Command(tcPfx+"gcc", "-c", "-mbig", "-mcpu=power10", "-x", "assembler-with-cpp", "-o", name+".o", "-")
	input, _ := cmd.StdinPipe()
	cmd.Stderr = os.Stderr
	go func() {
		defer input.Close()
		generator(input.(io.Writer))
	}()
	if cmd.Run() != nil {
		fmt.Printf("Failed running gcc for: %s\n", name)
		return
	}
	defer os.Remove(name + ".o")
	cmd = exec.Command(tcPfx+"objdump", "-d", name+".o")

	// Run objdump and parse output into test format
	output, _ := cmd.StdoutPipe()
	defer output.Close()
	scanner := bufio.NewScanner(output)
	spacere := regexp.MustCompile("[[:space:]]+")
	outf, _ := os.Create(name + ".txt")
	defer outf.Close()
	if cmd.Start() != nil {
		fmt.Printf("Failed running objdump for: %s\n", name)
		return
	}

	pfx := ""
	dec := ""
	for scanner.Scan() {
		ln := spacere.Split(scanner.Text(), -1)
		if len(ln) >= 7 {
			opc := strings.Join(ln[2:6], "")
			if len(pfx) == 0 {
				dec = strings.Join(ln[6:], " ")
			}
			if v, _ := strconv.ParseInt(ln[2], 16, 16); v&0xFC == 0x04 {
				pfx = opc
				continue
			}
			fmt.Fprintf(outf, "%s%s|\tgnu\t%s\n", pfx, opc, dec)
			pfx = ""
		}

	}
	cmd.Wait()
}

// Generate representative instructions for all[1] instructions in pp64.csv.
//
// [1] See hack.h for a few minor, exceptional workarounds.
func emitGenerated(out io.Writer) {
	cmd := exec.Command("go", "run", "../ppc64map/map.go", "-fmt=asm", "../pp64.csv")
	cmdout, _ := cmd.Output()
	out.Write(cmdout)
}

// Produce generated test outputs.  This should be run every so often with
// new versions of objdump to ensure we stay up to date.
func main() {
	genOutput("decode_branch", "powerpc64le-linux-gnu-", emitBranches)
	genOutput("decode_generated", "powerpc64le-linux-gnu-", emitGenerated)
}
