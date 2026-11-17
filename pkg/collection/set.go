/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package collection

// Set is a set of values of type K. Use NewSet or NewSetWithSize to make a
// set.
type Set[K comparable] map[K]struct{}

// NewSet gives a set with the given elements.
func NewSet[K comparable](values ...K) Set[K] {
	set := NewSetWithSize[K](len(values))
	for _, value := range values {
		set.Add(value)
	}
	return set
}

// NewSetWithSize gives an empty set. The set reserves room for cardinality
// elements.
func NewSetWithSize[K comparable](cardinality int) Set[K] {
	return make(Set[K], cardinality)
}

// Add adds key to the set.
func (p Set[K]) Add(key K) {
	p[key] = struct{}{}
}

// Contains reports whether the set has key.
func (p Set[K]) Contains(key K) bool {
	_, found := p[key]
	return found
}
