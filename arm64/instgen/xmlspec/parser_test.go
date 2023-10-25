// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xmlspec

import (
	"flag"
	"testing"
)

var remoteData = flag.Bool("remote", false, "use remote data")

func getData(t *testing.T) []*Instruction {
	if *remoteData {
		xmlDir, err := GetArm64XMLSpec(t.TempDir(), ExpectedURL, ExpectedVersion)
		if err != nil {
			t.Fatalf("GetArm64XMLSpec failed: %v", err)
		}
		return ParseXMLFiles(xmlDir)
	}
	return ParseXMLFiles("testdata")
}

func TestParseXMLFiles(t *testing.T) {
	insts := getData(t)
	// Merely check the parser runs.
	t.Logf("Number of instructions: %d\n", len(insts))
}
