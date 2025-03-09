// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"iter"
	"path/filepath"
	"strings"
)

type lineInfo struct {
	Pos
	data []byte
}

type Pos struct {
	Path string
	Line int
}

func (p Pos) String() string {
	if p.Line == 0 {
		if p.Path == "" {
			return "?:?"
		}
		return p.Path
	} else if p.Path == "" {
		return fmt.Sprintf("?:%d", p.Line)
	}
	return fmt.Sprintf("%s:%d", p.Path, p.Line)
}

func (p Pos) ShortString() string {
	p2 := p
	p2.Path = filepath.Base(p.Path)
	return p2.String()
}

// readLines yields lines from r, with continuation lines folded, comments and
// trailing whitespace removed, and blank lines omitted.
//
// The returned lineInfo.data buffer may be reused between yields.
//
// If r has a Name() string method, this is used to populate lineInfo.Path.
func readLines(r io.Reader) iter.Seq2[lineInfo, error] {
	type Named interface {
		Name() string // Matches os.File
	}
	path := ""
	if f, ok := r.(Named); ok {
		path = f.Name()
	}

	s := bufio.NewScanner(r)
	return func(yield func(lineInfo, error) bool) {
		var info lineInfo
		info.Path = path
		var lineBuf []byte
		for s.Scan() {
			info.Line++

			lineBuf = append(lineBuf, s.Bytes()...)
			if len(lineBuf) > 0 && lineBuf[len(lineBuf)-1] == '\\' {
				// Continuation line. Drop the \ and keep reading.
				lineBuf = lineBuf[:len(lineBuf)-1]
				continue
			}
			// Remove comments and trailing whitespace
			if i := strings.IndexByte(string(lineBuf), '#'); i >= 0 {
				lineBuf = lineBuf[:i]
			}
			lineBuf = bytes.TrimRight(lineBuf, " \t")
			// Don't yield blank lines
			if len(lineBuf) == 0 {
				continue
			}

			info.data = lineBuf
			if !yield(info, nil) {
				return
			}
			lineBuf = lineBuf[:0]
		}

		if err := s.Err(); err != nil {
			yield(lineInfo{}, err)
			return
		}
		if len(lineBuf) > 0 {
			yield(lineInfo{}, fmt.Errorf("continuation line at EOF"))
		}
	}
}
