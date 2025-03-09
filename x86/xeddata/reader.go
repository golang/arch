// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"regexp"
	"strings"
)

// Reader reads enc/dec-instruction objects from XED datafile.
type Reader struct {
	r io.Reader

	// Initialized on first call to Read
	next func() (*Object, error, bool)
	stop func()
	err  error
}

// NewReader returns a new Reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// Read reads single XED instruction object from
// the stream backed by reader.
//
// If there is no data left to be read,
// returned error is io.EOF.
func (r *Reader) Read() (*Object, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.next == nil {
		r.next, r.stop = iter.Pull2(readObjects(r.r))
	}
	obj, err, end := r.next()
	if end {
		err = io.EOF
	}
	if err != nil {
		r.stop()
		r.err, r.next, r.stop = err, nil, nil
		return nil, err
	}
	return obj, nil
}

// ReadAll reads all the remaining objects from r.
// A successful call returns err == nil, not err == io.EOF,
// just like csv.Reader.ReadAll().
func (r *Reader) ReadAll() ([]*Object, error) {
	var objects []*Object
	for obj, err := range readObjects(r.r) {
		if err != nil {
			return objects, err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// readObjects yields all of the objects from r.
func readObjects(r io.Reader) iter.Seq2[*Object, error] {
	iterLines := readLines(r)
	return func(yield func(*Object, error) bool) {
		var blockPos Pos
		var block []string // Reused on each iteration
		var linePos []Pos
		inBlock := false
		for line, err := range iterLines {
			if err != nil {
				yield(nil, err)
				return
			}
			if !inBlock {
				inBlock = line.data[0] == '{'
				blockPos = line.Pos
			} else if line.data[0] == '}' {
				inBlock = false
				obj, err := parseObjectLines(blockPos, block, linePos)
				if !yield(obj, err) {
					return
				}
				block, linePos = block[:0], linePos[:0]
			} else {
				block = append(block, string(line.data))
				linePos = append(linePos, line.Pos)
			}
		}
		if inBlock {
			yield(nil, errors.New("no matching '}' found"))
		}
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
var instLineRE = regexp.MustCompile(`^([A-Z_]+)\s*:\s*(.*)`)

// parseLines turns collected object lines into Object.
func parseObjectLines(blockPos Pos, lines []string, linePos []Pos) (*Object, error) {
	o := &Object{}
	o.Pos = blockPos

	// Repeatable tokens.
	// We can not assign them eagerly, because these fields
	// are not guaranteed to follow strict order.
	var (
		operands []string
		iforms   []string
		patterns []string
		poses    []Pos
	)

	for i, l := range lines {
		l = strings.TrimLeft(l, " ")
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
			poses = append(poses, linePos[i])
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
			Pos:      poses[i],
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
