// @generated AUTO GENERATED - DO NOT EDIT! 117d51fa2854b0184adc875246a35929bbbf0a91

// Copyright (c) 2018 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package orderings

import (
	"time"

	"github.com/svenskmand/mimir-lib/generation"
	gPlacement "github.com/svenskmand/mimir-lib/generation/placement"
	"github.com/svenskmand/mimir-lib/model/placement"
)

// OrderingBuilder is used to generate new orderings for use in tests and benchmarks.
type OrderingBuilder interface {
	// Generate will generate a custom ordering that depends on the random source and the time.
	Generate(random generation.Random, time time.Duration) placement.Ordering
}

// NewOrderingBuilder creates a new builder for building orderings.
func NewOrderingBuilder(builder OrderingBuilder) gPlacement.OrderingBuilder {
	return &orderingBuilder{
		subBuilder: builder,
	}
}

type orderingBuilder struct {
	subBuilder OrderingBuilder
}

func (builder *orderingBuilder) Generate(random generation.Random, time time.Duration) placement.Ordering {
	return builder.subBuilder.Generate(random, time)
}
