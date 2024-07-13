// Copyright 2024 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

// Generate interesting test cases from s390x objdump via
// go run util.go
//
// This requires "/usr/bin/gcc" and "objdump" be in the PATH this command is run.
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

// Emit a test file using the generator called name.txt.  This requires
// a GCC toolchain which supports -march=z16.
func genOutput(name, tcPfx string, generator func(io.Writer)) {
	// Generate object code from gcc
	cmd := exec.Command(tcPfx+"gcc", "-c", "-march=z16", "-x", "assembler-with-cpp", "-o", name+".o", "-")
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

	for scanner.Scan() {
		ln := spacere.Split(scanner.Text(), -1)
		var cnt int16
		if len(ln) >= 5 {
			v, _ := strconv.ParseInt(ln[2], 16, 16)
			if (v >> 6 & 0x3) == 0 {
				cnt = 2
			} else if v>>6&0x3 == 1 || v>>6&0x3 == 2 {
				cnt = 4
			} else {
				cnt = 6
			}
			opc := strings.Join(ln[2:cnt+2], "")
			dec := strings.Join(ln[cnt+2:], " ")
			fmt.Fprintf(outf, "%12s|\tgnu\t%-18s\n", opc, dec)
		}
	}
	cmd.Wait()
}

// Generate representative instructions for all[1] instructions in s390x.csv.
//
// [1] See hack.h for a few minor, exceptional workarounds.
func emitGenerated(out io.Writer) {
	cmd := exec.Command("go", "run", "../s390xmap/map.go", "-fmt=asm", "../s390x.csv")
	cmdout, _ := cmd.Output()
	out.Write(cmdout)
}

// Produce generated test outputs.  This should be run every so often with
// new versions of objdump to ensure we stay up to date.
func main() {
	genOutput("decode_generated", "/usr/bin/", emitGenerated)
}
