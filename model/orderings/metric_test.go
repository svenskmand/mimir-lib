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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/svenskmand/mimir-lib/internal"
	"github.com/svenskmand/mimir-lib/model/metrics"
	"github.com/svenskmand/mimir-lib/model/placement"
)

func TestOrderByMetricFreeDisk(t *testing.T) {
	ordering := Metric(GroupSource, metrics.DiskFree)
	group1, group2, groups, entity := internal.SetupTwoGroupsAndEntity()
	scopeSet := placement.NewScopeSet(groups)

	assert.True(t, placement.Less(ordering.Tuple(group1, scopeSet, entity), ordering.Tuple(group2, scopeSet, entity)))
}

func TestCustomByMultiplyNoSubExpressions(t *testing.T) {
	ordering := Multiply()

	group1, group2, groups, entity := internal.SetupTwoGroupsAndEntity()
	scopeSet := placement.NewScopeSet(groups)

	assert.Equal(t, 0, len(ordering.Tuple(group1, scopeSet, entity)))
	assert.Equal(t, 0, len(ordering.Tuple(group2, scopeSet, entity)))
}

func TestCustomByMetric(t *testing.T) {
	ordering := Multiply(
		Metric(GroupSource, metrics.DiskFree),
		Inverse(Metric(EntitySource, metrics.DiskUsed)),
	)
	group1, group2, groups, entity := internal.SetupTwoGroupsAndEntity()
	scopeSet := placement.NewScopeSet(groups)

	assert.Equal(t, 2.0, ordering.Tuple(group1, scopeSet, entity)[0])
	assert.Equal(t, 3.0, ordering.Tuple(group2, scopeSet, entity)[0])
}

func TestCustomByMetricForInvalidSource(t *testing.T) {
	ordering := Metric(Source("invalid"), metrics.DiskFree)
	group1, group2, groups, entity := internal.SetupTwoGroupsAndEntity()
	scopeSet := placement.NewScopeSet(groups)

	assert.Equal(t, 0.0, ordering.Tuple(group1, scopeSet, entity)[0])
	assert.Equal(t, 0.0, ordering.Tuple(group2, scopeSet, entity)[0])
}
