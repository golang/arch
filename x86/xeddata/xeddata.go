// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"os"
	"path/filepath"
)

// WalkInsts calls visit function for each XED instruction found at $xedPath/all-dec-instructions.txt.
func WalkInsts(xedPath string, visit func(*Inst)) error {
	f, err := os.Open(filepath.Join(xedPath, "all-dec-instructions.txt"))
	if err != nil {
		return err
	}
	for obj, err := range readObjects(f) {
		if err != nil {
			return err
		}
		for _, inst := range obj.Insts {
			visit(inst)
		}
	}
	return nil
}
