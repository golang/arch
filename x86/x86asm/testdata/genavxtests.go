// Copyright 2026 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// genavxtests generates test cases by running the assembler test
// files in reverse: given a binary encoding, expect the Plan 9/Go
// assembly input that produces the encoding. For GNU and Intel
// syntaxes, it uses objdump to retrieve the disassembly output.
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var tmpDir string

func main() {
	files := []string{
		runtime.GOROOT() + "/src/cmd/asm/internal/asm/testdata/amd64enc.s",
		runtime.GOROOT() + "/src/cmd/asm/internal/asm/testdata/amd64enc_extra.s",
	}

	// Assembler test is like
	// "  VPGATHERDQ Y2, (BP)(X7*2), Y1     // c4e2ed904c7d00"
	hexRe := regexp.MustCompile(`^\s*(.*?)\s*//\s*([0-9a-fA-F]+)\s*$`)
	type testCase struct {
		hex   string
		plan9 string
	}
	var testCases []testCase
	seen := make(map[string]bool)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			m := hexRe.FindStringSubmatch(line)
			if len(m) == 3 {
				plan9 := strings.TrimSpace(m[1])
				if plan9 == "" || strings.HasPrefix(plan9, "//") {
					continue
				}
				if !strings.HasPrefix(plan9, "V") && !strings.HasPrefix(plan9, "K") && !strings.HasPrefix(plan9, "SHA") {
					// Test only AVX and SHA instructions for now. (There exist tests for
					// non-AVX ones).
					continue
				}
				h := strings.ToLower(m[2])
				if !seen[h] {
					seen[h] = true
					testCases = append(testCases, testCase{hex: h, plan9: plan9})
				}
			}
		}
		f.Close()
	}

	var buf bytes.Buffer
	var err error
	tmpDir, err = os.MkdirTemp("", "avxtest")
	if err != nil {
		fmt.Println("Error creating temp dir:", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	for _, tc := range testCases {
		processHex(tc.hex, tc.plan9, &buf)
	}

	fmt.Println(buf.String())
}

func processHex(h string, plan9Asm string, buf *bytes.Buffer) {
	b, err := hex.DecodeString(h)
	if err != nil {
		fmt.Printf("Error decoding hex %s: %v\n", h, err)
		return
	}

	var s bytes.Buffer
	s.WriteString(".text\n")
	s.WriteString(".byte ")
	for i, v := range b {
		if i > 0 {
			s.WriteString(",")
		}
		fmt.Fprintf(&s, "0x%02x", v)
	}
	s.WriteString("\n")

	asmFile := filepath.Join(tmpDir, "testtmp.s")
	objFile := filepath.Join(tmpDir, "testtmp.o")

	err = os.WriteFile(asmFile, s.Bytes(), 0644)
	if err != nil {
		fmt.Println("Error writing testtmp.s:", err)
		return
	}

	cmd := exec.Command("cc", "-arch", "x86_64", "-c", asmFile, "-o", objFile)
	err = cmd.Run()
	if err != nil {
		fmt.Printf("Error compiling hex %s: %v\n", h, err)
		return
	}

	gnuOut, _ := exec.Command("objdump", "-d", objFile).CombinedOutput()
	intelOut, _ := exec.Command("objdump", "--x86-asm-syntax=intel", "-d", objFile).CombinedOutput()

	gnuAsm := parseObjdump(string(gnuOut))
	intelAsm := parseObjdump(string(intelOut))

	// Normalize spaces
	gnuAsm = strings.ReplaceAll(gnuAsm, "\t", " ")
	intelAsm = strings.ReplaceAll(intelAsm, "\t", " ")
	gnuAsm = strings.ReplaceAll(gnuAsm, ", ", ",")

	intelAsm = strings.ReplaceAll(intelAsm, " + ", "+")
	intelAsm = strings.ReplaceAll(intelAsm, " - ", "-")

	// Normalize Plan 9 decimal offsets to hex since x86asm outputs them as hex.
	rePlan9Num := regexp.MustCompile(`([\s\$])(-?[0-9]+)([\(,])`)
	plan9Asm = rePlan9Num.ReplaceAllStringFunc(plan9Asm, func(m string) string {
		matches := rePlan9Num.FindStringSubmatch(m)
		prefix := matches[1]
		numStr := matches[2]
		suffix := matches[3]
		n, _ := strconv.ParseInt(numStr, 10, 64)
		if prefix == "$" && n < 0 && n >= -128 {
			// Use unsigned immediate.
			// TODO: check.
			n = n & 0xff
		}
		if prefix != "$" && n == 0 { // 0 offset remains 0, not 0x0
			return fmt.Sprintf("%s0%s", prefix, suffix)
		}
		if n < 0 {
			return fmt.Sprintf("%s-0x%x%s", prefix, -n, suffix)
		}
		return fmt.Sprintf("%s0x%x%s", prefix, n, suffix)
	})

	rePlan9Mem := regexp.MustCompile(` \(([A-Z0-9]+)\)`)
	plan9Asm = rePlan9Mem.ReplaceAllString(plan9Asm, " 0(${1})")

	plan9Asm = strings.ReplaceAll(plan9Asm, ",", ", ")
	plan9Asm = strings.ReplaceAll(plan9Asm, ",  ", ", ")

	padSize := 16 - len(b)
	padding := ""
	if padSize > 0 {
		padding = strings.Repeat("5f", padSize)
	}
	hexStr := fmt.Sprintf("%s|%s", h, padding)

	buf.WriteString(fmt.Sprintf("%s\t64\tplan9\t%s\n", hexStr, plan9Asm))
	buf.WriteString(fmt.Sprintf("%s\t64\tgnu\t%s\n", hexStr, gnuAsm))
	buf.WriteString(fmt.Sprintf("%s\t64\tintel\t%s\n", hexStr, intelAsm))
}

// parseObjdump extracts the assembly string from objdump output.
// It expects lines like: "  0:  62 f1 fd 48 2d c2     vaddpd %zmm2,%zmm21,%zmm2"
func parseObjdump(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, ":") && (strings.Contains(line, "\t") || strings.Contains(line, "  ")) {
			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
			if len(parts) < 2 {
				continue
			}
			body := strings.TrimSpace(parts[1])
			var asm string
			subParts := strings.SplitN(body, "\t", 2)
			if len(subParts) == 2 {
				asm = strings.TrimSpace(subParts[1])
			} else {
				idx := strings.Index(body, "  ")
				if idx > 0 {
					asm = strings.TrimSpace(body[idx:])
				} else {
					continue
				}
			}
			asm = strings.ReplaceAll(asm, "\t", " ")
			if idx := strings.Index(asm, " ##"); idx >= 0 {
				asm = strings.TrimSpace(asm[:idx])
			}
			if idx := strings.Index(asm, " #"); idx >= 0 {
				asm = strings.TrimSpace(asm[:idx])
			}
			if idx := strings.Index(asm, " <"); idx >= 0 {
				asm = strings.TrimSpace(asm[:idx])
			}
			return asm
		}
	}
	return ""
}
