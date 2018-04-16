// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xeddata

import (
	"sort"
	"strings"
)

// PatternSet wraps instruction PATTERN properties providing set operations on them.
type PatternSet map[string]bool

// NewPatternSet decodes pattern string into PatternSet.
func NewPatternSet(pattern string) PatternSet {
	pset := make(PatternSet)
	for _, f := range strings.Fields(pattern) {
		pset[f] = true
	}
	return pset
}

// PatternAliases is extendable map of pattern keys aliases.
// Maps human-readable key to XED property.
//
// Used in PatternSet.Is.
var PatternAliases = map[string]string{
	"VEX":     "VEXVALID=1",
	"EVEX":    "VEXVALID=2",
	"XOP":     "VEXVALID=3",
	"MemOnly": "MOD!=3",
	"RegOnly": "MOD=3",
}

// String returns pattern printer representation.
// All properties are sorted.
func (pset PatternSet) String() string {
	var keys []string
	for k := range pset {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

// Is reports whether set contains key k.
// In contrast with direct pattern set lookup, it does
// check if PatternAliases[k] is available to be used instead of k in lookup.
func (pset PatternSet) Is(k string) bool {
	if alias := PatternAliases[k]; alias != "" {
		return pset[alias]
	}
	return pset[k]
}

// Replace inserts newKey if oldKey is defined.
// oldKey is removed if insertion is performed.
func (pset PatternSet) Replace(oldKey, newKey string) {
	if pset[oldKey] {
		pset[newKey] = true
		delete(pset, oldKey)
	}
}

// Index returns index from keys of first matching key.
// Returns -1 if does not contain any of given keys.
func (pset PatternSet) Index(keys ...string) int {
	for i, k := range keys {
		if pset[k] {
			return i
		}
	}
	return -1
}

// Match is like MatchOrDefault("", keyval...).
func (pset PatternSet) Match(keyval ...string) string {
	return pset.MatchOrDefault("", keyval...)
}

// MatchOrDefault returns first matching key associated value.
// Returns defaultValue if no match is found.
//
// Keyval structure can be described as {"k1", "v1", ..., "kN", "vN"}.
func (pset PatternSet) MatchOrDefault(defaultValue string, keyval ...string) string {
	for i := 0; i < len(keyval); i += 2 {
		key := keyval[i+0]
		val := keyval[i+1]
		if pset[key] {
			return val
		}
	}
	return defaultValue
}
