// @generated AUTO GENERATED - DO NOT EDIT! 9f8b9e47d86b5e1a3668856830c149e768e78415
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

package generation

import (
	"time"

	"code.uber.internal/infra/peloton/mimir-lib/model/labels"
	"code.uber.internal/infra/peloton/mimir-lib/model/placement"
	"code.uber.internal/infra/peloton/mimir-lib/model/requirements"
)

// NewLabelRequirementBuilder will create a new label requirement builder requiring that the labels occurrences to
// fulfill the comparison.
func NewLabelRequirementBuilder(scope, label labels.LabelTemplate, comparison requirements.Comparison,
	occurrences int) RequirementBuilder {
	return &labelRequirementBuilder{
		scope:       scope,
		label:       label,
		comparison:  comparison,
		occurrences: occurrences,
	}
}

type labelRequirementBuilder struct {
	scope       labels.LabelTemplate
	label       labels.LabelTemplate
	comparison  requirements.Comparison
	occurrences int
}

func (builder *labelRequirementBuilder) Generate(random Random, time time.Duration) placement.Requirement {
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
