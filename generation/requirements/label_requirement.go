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

package requirements

import (
	"time"

	"github.com/svenskmand/mimir-lib/generation"
	gPlacement "github.com/svenskmand/mimir-lib/generation/placement"
	"github.com/svenskmand/mimir-lib/model/labels"
	mPlacement "github.com/svenskmand/mimir-lib/model/placement"
	"github.com/svenskmand/mimir-lib/model/requirements"
)

// NewLabelRequirementBuilder will create a new label requirement builder requiring that the labels occurrences to
// fulfill the comparison.
func NewLabelRequirementBuilder(scope, label labels.Template, comparison requirements.Comparison,
	occurrences int) gPlacement.RequirementBuilder {
	return &labelRequirementBuilder{
		scope:       scope,
		label:       label,
		comparison:  comparison,
		occurrences: occurrences,
	}
}

type labelRequirementBuilder struct {
	scope       labels.Template
	label       labels.Template
	comparison  requirements.Comparison
	occurrences int
}

func (builder *labelRequirementBuilder) Generate(random generation.Random, time time.Duration) mPlacement.Requirement {
	var scope *labels.Label
	if builder.scope != nil {
		scope = builder.scope.Instantiate()
	}
	return requirements.NewLabelRequirement(
		scope,
		builder.label.Instantiate(),
		builder.comparison,
		builder.occurrences,
	)
}
