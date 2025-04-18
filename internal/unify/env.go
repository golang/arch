// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unify

import (
	"fmt"
	"iter"
	"reflect"
	"slices"
	"strings"
)

// A nonDetEnv is a non-deterministic mapping from [ident]s to [Value]s.
//
// Logically, this is just a set of deterministic environments, where each
// deterministic environment is a complete mapping from each [ident]s to exactly
// one [Value]. In particular, [ident]s are NOT necessarily independent of each
// other. For example, an environment may have both {x: 1, y: 1} and {x: 2, y:
// 2}, but not {x: 1, y: 2}.
//
// A nonDetEnv is immutable.
//
// Often [ident]s are independent of each other, so the representation optimizes
// for this by using a cross-product of environment factors, where each factor
// is a sum of deterministic environments. These operations obey the usual
// distributional laws, so we can always canonicalize into this form. (It MAY be
// worthwhile to allow more general expressions of sums and products.)
//
// For example, to represent {{x: 1, y: 1}, {x: 2, y: 2}}, in which the
// variables x and y are dependent, we need a single factor that covers x and y
// and consists of two terms: {x: 1, y: 1} + {x: 2, y: 2}.
//
// If we add a third variable z that can be 1 or 2, independent of x and y, we
// get four logical environments:
//
//	{x: 1, y: 1, z: 1}
//	{x: 2, y: 2, z: 1}
//	{x: 1, y: 1, z: 2}
//	{x: 2, y: 2, z: 2}
//
// This could be represented as a single factor that is the sum of these four
// detEnvs, but because z is independent, it can be a separate factor. Hence,
// the most compact representation of this environment is:
//
//	({x: 1, y: 1} + {x: 2, y: 2}) ⨯ ({z: 1} + {z: 2})
//
// That is, two factors, where each is the sum of two terms.
type nonDetEnv struct {
	// factors is a list of the multiplicative factors in this environment. The
	// set of deterministic environments is the cross-product of these factors.
	// All factors must have disjoint variables.
	factors []*envSum
}

// envSum is a sum of deterministic environments, all with the same set of
// variables.
type envSum struct {
	ids   []*ident // TODO: Do we ever use this as a slice? Should it be a map?
	terms []detEnv
}

type detEnv struct {
	vals []*Value // Indexes correspond to envSum.ids
}

var (
	// zeroEnvFactor is the "0" value of an [envSum]. It's a a factor with no
	// sum terms. This is easiest to think of as: an empty sum must be the
	// additive identity, 0.
	zeroEnvFactor = &envSum{}

	// topEnv is the algebraic one value of a [nonDetEnv]. It has no factors
	// because the product of no factors is the multiplicative identity.
	topEnv = nonDetEnv{}
	// bottomEnv is the algebraic zero value of a [nonDetEnv]. The product of
	// bottomEnv with x is bottomEnv, and the sum of bottomEnv with y is y.
	bottomEnv = nonDetEnv{factors: []*envSum{zeroEnvFactor}}
)

// bind binds id to each of vals in e.
//
// Its panics if id is already bound in e.
//
// Environments are typically initially constructed by starting with [topEnv]
// and calling bind one or more times.
func (e nonDetEnv) bind(id *ident, vals ...*Value) nonDetEnv {
	if e.isBottom() {
		return bottomEnv
	}

	// TODO: If any of vals are _, should we just not do anything? We're kind of
	// inconsistent about whether an id missing from e means id is invalid or
	// means id is _.

	// Check that id isn't present in e.
	for _, f := range e.factors {
		if slices.Contains(f.ids, id) {
			panic("id " + id.name + " already present in environment")
		}
	}

	// Create the new sum term.
	sum := &envSum{ids: []*ident{id}}
	for _, val := range vals {
		sum.terms = append(sum.terms, detEnv{vals: []*Value{val}})
	}
	// Multiply it in.
	factors := append(e.factors[:len(e.factors):len(e.factors)], sum)
	return nonDetEnv{factors}
}

func (e nonDetEnv) isBottom() bool {
	if len(e.factors) == 0 {
		// This is top.
		return false
	}
	return len(e.factors[0].terms) == 0
}

func (e nonDetEnv) vars() iter.Seq[*ident] {
	return func(yield func(*ident) bool) {
		for _, t := range e.factors {
			for _, id := range t.ids {
				if !yield(id) {
					return
				}
			}
		}
	}
}

// all enumerates all deterministic environments in e.
//
// The result slice is in the same order as the slice returned by
// [nonDetEnv2.vars]. The slice is reused between iterations.
func (e nonDetEnv) all() iter.Seq[[]*Value] {
	return func(yield func([]*Value) bool) {
		var vals []*Value
		var walk func(int) bool
		walk = func(i int) bool {
			if i == len(e.factors) {
				return yield(vals)
			}
			start := len(vals)
			for _, term := range e.factors[i].terms {
				vals = append(vals[:start], term.vals...)
				if !walk(i + 1) {
					return false
				}
			}
			return true
		}
		walk(0)
	}
}

// allOrdered is like all, but idOrder controls the order of the values in the
// resulting slice. Any [ident]s in idOrder that are missing from e are set to
// topValue. The values of idOrder must be a bijection with [0, n).
func (e nonDetEnv) allOrdered(idOrder map[*ident]int) iter.Seq[[]*Value] {
	valsLen := 0
	for _, idx := range idOrder {
		valsLen = max(valsLen, idx+1)
	}

	return func(yield func([]*Value) bool) {
		vals := make([]*Value, valsLen)
		// e may not have all of the IDs in idOrder. Make sure any missing
		// values are top.
		for i := range vals {
			vals[i] = topValue
		}
		var walk func(int) bool
		walk = func(i int) bool {
			if i == len(e.factors) {
				return yield(vals)
			}
			for _, term := range e.factors[i].terms {
				for j, id := range e.factors[i].ids {
					vals[idOrder[id]] = term.vals[j]
				}
				if !walk(i + 1) {
					return false
				}
			}
			return true
		}
		walk(0)
	}
}

func crossEnvs(envs ...nonDetEnv) nonDetEnv {
	// Combine the factors of envs
	var factors []*envSum
	haveIDs := map[*ident]struct{}{}
	for _, e := range envs {
		if e.isBottom() {
			// The environment is bottom, so the whole product goes to
			// bottom.
			return bottomEnv
		}
		// Check that all ids are disjoint.
		for _, f := range e.factors {
			for _, id := range f.ids {
				if _, ok := haveIDs[id]; ok {
					panic("conflict on " + id.name)
				}
				haveIDs[id] = struct{}{}
			}
		}
		// Everything checks out. Multiply the factors.
		factors = append(factors, e.factors...)
	}
	return nonDetEnv{factors: factors}
}

func sumEnvs(envs ...nonDetEnv) nonDetEnv {
	// nonDetEnv is a product at the top level, so we implement summation using
	// the distributive law. We also use associativity to keep as many top-level
	// factors as we can, since those are what keep the environment compact.
	//
	// a * b * c + a * d         (where a, b, c, and d are factors)
	//                           (combine common factors)
	//   = a * (b * c + d)
	//                           (expand factors into their sum terms)
	//   = a * ((b_1 + b_2 + ...) * (c_1 + c_2 + ...) + d)
	//                           (where b_i and c_i are deterministic environments)
	//                           (FOIL)
	//   = a * (b_1 * c_1 + b_1 * c_2 + b_2 * c_1 + b_2 * c2 + d)
	//                           (all factors are now in canonical form)
	//   = a * e
	//
	// The product of two deterministic environments is a deterministic
	// environment, and the sum of deterministic environments is a factor, so
	// this process results in the canonical product-of-sums form.
	//
	// TODO: This is a bit of a one-way process. We could try to factor the
	// environment to reduce the number of sums. I'm not sure how to do this
	// efficiently. It might be possible to guide it by gathering the
	// distributions of each ID's bindings. E.g., if there are 12 deterministic
	// environments in a sum and $x is bound to 4 different values, each 3
	// times, then it *might* be possible to factor out $x into a 4-way sum of
	// its own.

	factors, toSum := commonFactors(envs)

	if len(toSum) > 0 {
		// Collect all IDs into a single order.
		var ids []*ident
		idOrder := make(map[*ident]int)
		for _, e := range toSum {
			for v := range e.vars() {
				if _, ok := idOrder[v]; !ok {
					idOrder[v] = len(ids)
					ids = append(ids, v)
				}
			}
		}

		// Flatten out each term in the sum.
		var summands []detEnv
		for _, env := range toSum {
			for vals := range env.allOrdered(idOrder) {
				summands = append(summands, detEnv{vals: slices.Clone(vals)})
			}
		}
		factors = append(factors, &envSum{ids: ids, terms: summands})
	}

	return nonDetEnv{factors: factors}
}

// commonFactors finds common factors that can be factored out of a summation of
// [nonDetEnv]s.
func commonFactors(envs []nonDetEnv) (common []*envSum, toSum []nonDetEnv) {
	// Drop any bottom environments. They don't contribute to the sum and they
	// would complicate some logic below.
	envs = slices.DeleteFunc(envs, func(e nonDetEnv) bool {
		return e.isBottom()
	})
	if len(envs) == 0 {
		return bottomEnv.factors, nil
	}

	// It's very common that the exact same factor will appear across all envs.
	// Keep those factored out.
	//
	// TODO: Is it also common to have vars that are bound to the same value
	// across all envs? If so, we could also factor those into common terms.
	counts := map[*envSum]int{}
	for _, e := range envs {
		for _, f := range e.factors {
			counts[f]++
		}
	}
	for _, f := range envs[0].factors {
		if counts[f] == len(envs) {
			// Common factor
			common = append(common, f)
		}
	}

	// Any other factors need to be multiplied out.
	for _, env := range envs {
		var newFactors []*envSum
		for _, f := range env.factors {
			if counts[f] != len(envs) {
				newFactors = append(newFactors, f)
			}
		}
		if len(newFactors) > 0 {
			toSum = append(toSum, nonDetEnv{factors: newFactors})
		}
	}

	return common, toSum
}

// envPartition is a subset of an env where id is bound to value in all
// deterministic environments.
type envPartition struct {
	id    *ident
	value *Value
	env   nonDetEnv
}

func (e nonDetEnv) partitionBy(id *ident) []envPartition {
	if e.isBottom() {
		// Bottom contains all variables
		return []envPartition{{id: id, value: bottomValue, env: e}}
	}

	// Find the factor containing id and id's index in that factor.
	idFactor, idIndex := -1, -1
	var newIDs []*ident
	for factI, fact := range e.factors {
		idI := slices.Index(fact.ids, id)
		if idI < 0 {
			continue
		} else if idFactor != -1 {
			panic("multiple factors containing id " + id.name)
		} else {
			idFactor, idIndex = factI, idI
			// Drop id from this factor's IDs
			newIDs = without(fact.ids, idI)
		}
	}
	if idFactor == -1 {
		panic("id " + id.name + " not found in environment")
	}

	// If id is the only term in its factor, then dropping it is equivalent to
	// making the factor be the unit value, so we can just drop the factor. (And
	// if this is the only factor, we'll arrive at [topEnv], which is exactly
	// what we want!). In this case we can use the same nonDetEnv in all of the
	// partitions.
	isUnit := len(newIDs) == 0
	var unitFactors []*envSum
	if isUnit {
		unitFactors = without(e.factors, idFactor)
	}

	// Create a partition for each distinct value of id.
	var parts []envPartition
	partIndex := map[*Value]int{}
	for _, det := range e.factors[idFactor].terms {
		val := det.vals[idIndex]
		i, ok := partIndex[val]
		if !ok {
			i = len(parts)
			var factors []*envSum
			if isUnit {
				factors = unitFactors
			} else {
				// Copy all other factor
				factors = slices.Clone(e.factors)
				factors[idFactor] = &envSum{ids: newIDs}
			}
			parts = append(parts, envPartition{id: id, value: val, env: nonDetEnv{factors: factors}})
			partIndex[val] = i
		}

		if !isUnit {
			factor := parts[i].env.factors[idFactor]
			newVals := without(det.vals, idIndex)
			factor.terms = append(factor.terms, detEnv{vals: newVals})
		}
	}
	return parts
}

type ident struct {
	_    [0]func() // Not comparable (only compare *ident)
	name string
}

type Var struct {
	id *ident
}

func (d Var) Exact() bool {
	// These can't appear in concrete Values.
	panic("Exact called on non-concrete Value")
}

func (d Var) decode(rv reflect.Value) error {
	return &inexactError{"var", rv.Type().String()}
}

func (d Var) unify(w *Value, e nonDetEnv, swap bool, uf *unifier) (Domain, nonDetEnv, error) {
	// TODO: Vars from !sums in the input can have a huge number of values.
	// Unifying these could be way more efficient with some indexes over any
	// exact values we can pull out, like Def fields that are exact Strings.
	// Maybe we try to produce an array of yes/no/maybe matches and then we only
	// have to do deeper evaluation of the maybes. We could probably cache this
	// on an envTerm. It may also help to special-case Var/Var unification to
	// pick which one to index versus enumerate.

	if vd, ok := w.Domain.(Var); ok && d.id == vd.id {
		// Unifying $x with $x results in $x. If we descend into this we'll have
		// problems because we strip $x out of the environment to keep ourselves
		// honest and then can't find it on the other side.
		//
		// TODO: I'm not positive this is the right fix.
		return vd, e, nil
	}

	// We need to unify w with the value of d in each possible environment. We
	// can save some work by grouping environments by the value of d, since
	// there will be a lot of redundancy here.
	var nEnvs []nonDetEnv
	envParts := e.partitionBy(d.id)
	for i, envPart := range envParts {
		exit := uf.enterVar(d.id, i)
		// Each branch logically gets its own copy of the initial environment
		// (narrowed down to just this binding of the variable), and each branch
		// may result in different changes to that starting environment.
		res, e2, err := w.unify(envPart.value, envPart.env, swap, uf)
		exit.exit()
		if err != nil {
			return nil, nonDetEnv{}, err
		}
		if res.Domain == nil {
			// This branch entirely failed to unify, so it's gone.
			continue
		}
		nEnv := e2.bind(d.id, res)
		nEnvs = append(nEnvs, nEnv)
	}

	if len(nEnvs) == 0 {
		// All branches failed
		return nil, bottomEnv, nil
	}

	// The effect of this is entirely captured in the environment. We can return
	// back the same Bind node.
	return d, sumEnvs(nEnvs...), nil
}

// An identPrinter maps [ident]s to unique string names.
type identPrinter struct {
	ids   map[*ident]string
	idGen map[string]int
}

func (p *identPrinter) unique(id *ident) string {
	if p.ids == nil {
		p.ids = make(map[*ident]string)
		p.idGen = make(map[string]int)
	}

	name, ok := p.ids[id]
	if !ok {
		gen := p.idGen[id.name]
		p.idGen[id.name]++
		if gen == 0 {
			name = id.name
		} else {
			name = fmt.Sprintf("%s#%d", id.name, gen)
		}
		p.ids[id] = name
	}

	return name
}

func (p *identPrinter) slice(ids []*ident) string {
	var strs []string
	for _, id := range ids {
		strs = append(strs, p.unique(id))
	}
	return fmt.Sprintf("[%s]", strings.Join(strs, ", "))
}

func without[Elt any](s []Elt, i int) []Elt {
	return append(s[:i:i], s[i+1:]...)
}
