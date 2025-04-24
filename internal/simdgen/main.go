// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// simdgen is an experiment in generating Go <-> asm SIMD mappings.
//
// Usage: simdgen [-xedPath=path] [-q=query] input.yaml...
//
// If -xedPath is provided, one of the inputs is a sum of op-code definitions
// generated from the Intel XED data at path.
//
// If input YAML files are provided, each file is read as an input value. See
// [unify.Closure.UnmarshalYAML] or "go doc unify.Closure.UnmarshalYAML" for the
// format of these files.
//
// TODO: Example definitions and values.
//
// The command unifies across all of the inputs and prints all possible results
// of this unification.
//
// If the -q flag is provided, its string value is parsed as a value and treated
// as another input to unification. This is intended as a way to "query" the
// result, typically by narrowing it down to a small subset of results.
//
// Typical usage:
//
//	go run . -xedPath $XEDPATH *.yaml
//
// To see just the definitions generated from XED, run:
//
//	go run . -xedPath $XEDPATH
//
// (This works because if there's only one input, there's nothing to unify it
// with, so the result is simply itself.)
//
// To see just the definitions for VPADDQ:
//
//	go run . -xedPath $XEDPATH -q '{asm: VPADDQ}'
package main

// Big TODOs:
//
// - This can produce duplicates, which can also lead to less efficient
// environment merging. Add hashing and use it for deduplication. Be careful
// about how this shows up in debug traces, since it could make things
// confusing if we don't show it happening.
//
// - Do I need Closure, Value, and Domain? It feels like I should only need two
// types.

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"strings"

	"golang.org/x/arch/internal/unify"
	"gopkg.in/yaml.v3"
)

var (
	xedPath = flag.String("xedPath", "", "load XED datafiles from `path`")
	flagQ   = flag.String("q", "", "query: read `def` as another input (skips final validation)")
	flagO   = flag.String("o", "yaml", "output type: yaml, godefs")

	flagDebugXED   = flag.Bool("debug-xed", false, "show XED instructions")
	flagDebugUnify = flag.Bool("debug-unify", false, "print unification trace")
	flagDebugHTML  = flag.String("debug-html", "", "write unification trace to `file.html`")
)

var yamlSubs = strings.NewReplacer(
	"$xi", "[BWDQ]", // x86 integer suffixes
	"$xf", "[SD]", // x86 float suffixes
)

func main() {
	flag.Parse()

	var inputs []unify.Closure

	// Load XED into a defs set.
	if *xedPath != "" {
		xedDefs := loadXED(*xedPath)
		inputs = append(inputs, unify.NewSum(xedDefs...))
	}

	// Load query.
	if *flagQ != "" {
		r := strings.NewReader(*flagQ)
		var def unify.Closure
		if err := def.Unmarshal(r, unify.UnmarshalOpts{Path: "<query>", StringReplacer: yamlSubs.Replace}); err != nil {
			log.Fatalf("parsing -q: %s", err)
		}
		inputs = append(inputs, def)
	}

	// Load defs files.
	must := make(map[*unify.Value]struct{})
	for _, path := range flag.Args() {
		defs, err := loadValue(path)
		if err != nil {
			log.Fatal(err)
		}
		inputs = append(inputs, defs)

		if path == "go.yaml" {
			// These must all be used in the final result
			for def := range defs.Summands() {
				must[def] = struct{}{}
			}
		}
	}

	// Prepare for unification
	if *flagDebugUnify {
		unify.Debug.UnifyLog = os.Stderr
	}
	if *flagDebugHTML != "" {
		f, err := os.Create(*flagDebugHTML)
		if err != nil {
			log.Fatal(err)
		}
		unify.Debug.HTML = f
		defer f.Close()
	}

	// Unify!
	unified, err := unify.Unify(inputs...)
	if err != nil {
		log.Fatal(err)
	}

	// Print results.
	switch *flagO {
	case "yaml":
		// Produce a result that looks like encoding a slice, but stream it.
		var val1 [1]*unify.Value
		for val := range unified.All() {
			val1[0] = val
			// We have to make a new encoder each time or it'll print a document
			// separator between each object.
			enc := yaml.NewEncoder(os.Stdout)
			if err := enc.Encode(val1); err != nil {
				log.Fatal(err)
			}
			enc.Close()
		}
	case "godefs":
		writeGoDefs(os.Stdout, unified)
	}

	// Validate results.
	//
	// Don't validate if this is a command-line query because that tends to
	// eliminate lots of required defs and is used in cases where maybe defs
	// aren't enumerable anyway.
	if *flagQ == "" && len(must) > 0 {
		validate(unified, must)
	}
}

func loadValue(path string) (unify.Closure, error) {
	f, err := os.Open(path)
	if err != nil {
		return unify.Closure{}, err
	}
	defer f.Close()

	var c unify.Closure
	if err := c.Unmarshal(f, unify.UnmarshalOpts{StringReplacer: yamlSubs.Replace}); err != nil {
		return unify.Closure{}, fmt.Errorf("%s: %v", path, err)
	}
	return c, nil
}

func validate(cl unify.Closure, required map[*unify.Value]struct{}) {
	// Validate that:
	// 1. All final defs are exact
	// 2. All required defs are used
	for def := range cl.All() {
		if _, ok := def.Domain.(unify.Def); !ok {
			fmt.Fprintf(os.Stderr, "%s: expected Def, got %T\n", def.PosString(), def.Domain)
			continue
		}

		if !def.Exact() {
			fmt.Fprintf(os.Stderr, "%s: def not reduced to an exact value:\n", def.PosString())
			fmt.Fprintf(os.Stderr, "\t%s\n", strings.ReplaceAll(def.String(), "\n", "\n\t"))
		}

		for root := range def.Provenance() {
			delete(required, root)
		}
	}
	// Report unused defs
	unused := slices.SortedFunc(maps.Keys(required),
		func(a, b *unify.Value) int {
			return cmp.Or(
				cmp.Compare(a.Pos().Path, b.Pos().Path),
				cmp.Compare(a.Pos().Line, b.Pos().Line),
			)
		})
	for _, def := range unused {
		// TODO: Can we say anything more actionable? This is always a problem
		// with unification: if it fails, it's very hard to point a finger at
		// any particular reason. We could go back and try unifying this again
		// with each subset of the inputs (starting with individual inputs) to
		// at least say "it doesn't unify with anything in x.yaml". That's a lot
		// of work, but if we have trouble debugging unification failure it may
		// be worth it.
		fmt.Fprintf(os.Stderr, "%s: def required, but did not unify\n", def.PosString())
	}
}
