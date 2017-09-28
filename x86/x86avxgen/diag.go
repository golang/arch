// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// These diagnostics were extensively used during development phase.
// Now they serve as additional level of tests.
// If coverage is not 100% for any reason, troubleshooting is required.

import (
	"fmt"
	"sort"
)

// diagnostics is used to collect and display execution info.
type diagnostics struct {
	// Count misses for undefined ytab key.
	ytabMisses      map[string]int
	optabsGenerated int
	optabsTotal     int
}

func (d *diagnostics) Print() {
	fmt.Println("  -- diag info --")
	d.printOptabsInfo()
	fmt.Println()
	d.printYtabMisses()
}

func (d *diagnostics) printOptabsInfo() {
	skipped := d.optabsTotal - d.optabsGenerated
	cover := float64(d.optabsGenerated*100) / float64(d.optabsTotal)
	fmt.Println("Optabs info:")
	fmt.Printf("  processed: %d\n", d.optabsTotal)
	fmt.Printf("  generated: %d\n", d.optabsGenerated)
	fmt.Printf("    skipped: %d\n", skipped)
	fmt.Printf("      cover: %.1f%%\n", cover)
}

func (d *diagnostics) printYtabMisses() {
	if len(d.ytabMisses) == 0 {
		fmt.Println("No ytab key misses recorded")
		return
	}

	// Sort by miss count.
	type ytabMiss struct {
		key   string
		count int
	}
	misses := make([]ytabMiss, 0, len(d.ytabMisses))
	for key, count := range d.ytabMisses {
		misses = append(misses, ytabMiss{
			key:   key,
			count: count,
		})
	}
	sort.Slice(misses, func(i, j int) bool {
		return misses[i].count > misses[j].count
	})

	fmt.Println("Missed ytab keys:")
	for _, m := range misses {
		fmt.Printf("  %+40s = %d\n", m.key, m.count)
	}
}

var diag = diagnostics{
	ytabMisses: make(map[string]int),
}
