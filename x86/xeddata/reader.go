// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Reader reads enc/dec-instruction objects from XED datafile.
type Reader struct {
	scanner *bufio.Scanner

	lines []string // Re-used between Read calls

	// True if last line ends with newline escape (backslash).
	joinLines bool
}

// NewReader returns a new Reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return newReader(bufio.NewScanner(r))
}

func newReader(scanner *bufio.Scanner) *Reader {
	r := &Reader{
		lines:   make([]string, 0, 64),
		scanner: scanner,
	}
	scanner.Split(r.split)
	return r
}

// split implements bufio.SplitFunc for Reader.
func (r *Reader) split(data []byte, atEOF bool) (int, []byte, error) {
	// Wrapping bufio.ScanLines to handle \-style newline escapes.
	// joinLines flag affects Reader.scanLine behavior.
	advance, tok, err := bufio.ScanLines(data, atEOF)
	if err == nil && len(tok) >= 1 {
		r.joinLines = tok[len(tok)-1] == '\\'
	}
	return advance, tok, err
}

// Read reads single XED instruction object from
// the stream backed by reader.
//
// If there is no data left to be read,
// returned error is io.EOF.
func (r *Reader) Read() (*Object, error) {
	for line := r.scanLine(); line != ""; line = r.scanLine() {
		if line[0] != '{' {
			continue
		}
		lines := r.lines[:0] // Object lines
		for line := r.scanLine(); line != ""; line = r.scanLine() {
			if line[0] == '}' {
				return r.parseLines(lines)
			}
			lines = append(lines, line)
		}
		return nil, errors.New("no matching '}' found")
	}

	return nil, io.EOF
}

// ReadAll reads all the remaining objects from r.
// A successful call returns err == nil, not err == io.EOF,
// just like csv.Reader.ReadAll().
func (r *Reader) ReadAll() ([]*Object, error) {
	objects := []*Object{}
	for {
		o, err := r.Read()
		if err == io.EOF {
			return objects, nil
		}
		if err != nil {
			return objects, err
		}
		objects = append(objects, o)
	}
}

// instLineRE matches valid XED object/inst line.
// It expects lines that are joined by '\' to be concatenated.
//
// The format can be described as:
//
//	unquoted field name "[A-Z_]+" (captured)
//	field value delimiter ":"
//	field value string (captured)
//	optional trailing comment that is ignored "[^#]*"
var instLineRE = regexp.MustCompile(`^([A-Z_]+)\s*:\s*([^#]*)`)

// parseLines turns collected object lines into Object.
func (r *Reader) parseLines(lines []string) (*Object, error) {
	o := &Object{}

	// Repeatable tokens.
	// We can not assign them eagerly, because these fields
	// are not guaranteed to follow strict order.
	var (
		operands []string
		iforms   []string
		patterns []string
	)

	for _, l := range lines {
		if l[0] == '#' { // Skip comment lines.
			continue
		}
		m := instLineRE.FindStringSubmatch(l)
		if len(m) == 0 {
			return nil, fmt.Errorf("malformed line: %s", l)
		}
		key, val := m[1], m[2]
		val = strings.TrimSpace(val)

		switch key {
		case "ICLASS":
			o.Iclass = val
		case "DISASM":
			o.Disasm = val
		case "DISASM_INTEL":
			o.DisasmIntel = val
		case "DISASM_ATTSV":
			o.DisasmATTSV = val
		case "ATTRIBUTES":
			o.Attributes = val
		case "UNAME":
			o.Uname = val
		case "CPL":
			o.CPL = val
		case "CATEGORY":
			o.Category = val
		case "EXTENSION":
			o.Extension = val
		case "EXCEPTIONS":
			o.Exceptions = val
		case "ISA_SET":
			o.ISASet = val
		case "FLAGS":
			o.Flags = val
		case "COMMENT":
			o.Comment = val
		case "VERSION":
			o.Version = val
		case "REAL_OPCODE":
			o.RealOpcode = val

		case "OPERANDS":
			operands = append(operands, val)
		case "PATTERN":
			patterns = append(patterns, val)
		case "IFORM":
			iforms = append(iforms, val)

		default:
			// Being strict about unknown field names gives a nice
			// XED file validation diagnostics.
			// Also defends against typos in test files.
			return nil, fmt.Errorf("unknown key token: %s", key)
		}
	}

	if len(operands) != len(patterns) {
		return nil, fmt.Errorf("%s: OPERANDS and PATTERN lines mismatch", o.Opcode())
	}

	insts := make([]*Inst, len(operands))
	for i := range operands {
		insts[i] = &Inst{
			Object:   o,
			Index:    i,
			Pattern:  patterns[i],
			Operands: operands[i],
		}
		// There can be less IFORMs than insts.
		if i < len(iforms) {
			insts[i].Iform = iforms[i]
		}
	}
	o.Insts = insts

	return o, nil
}

// scanLine tries to fetch non-empty line from scanner.
//
// Returns empty line when scanner.Scan() returns false
// before non-empty line is found.
func (r *Reader) scanLine() string {
	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			continue
		}
		if r.joinLines {
			return line[:len(line)-len("\\")] + r.scanLine()
		}
		return line
	}
	return ""
}
