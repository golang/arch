// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"io"
	"os"
	"path/filepath"
)

// WalkInsts calls visit function for each XED instruction found at $xedPath/all-dec-instructions.txt.
func WalkInsts(xedPath string, visit func(*Inst)) error {
	f, err := os.Open(filepath.Join(xedPath, "all-dec-instructions.txt"))
	if err != nil {
		return err
	}
	r := NewReader(f)
	for {
		o, err := r.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		for _, inst := range o.Insts {
			visit(inst)
		}
	}
}
