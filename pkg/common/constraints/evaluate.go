// Copyright (c) 2019 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package constraints

import (
	"errors"

	"github.com/uber/peloton/.gen/peloton/api/v0/peloton"
	"github.com/uber/peloton/.gen/peloton/api/v0/task"

	"github.com/uber/peloton/pkg/common"

	log "github.com/sirupsen/logrus"
)

// Evaluator is the interface to evaluate whether given LabelValueSet satisifies
// given constraint.
// 评估接口
type Evaluator interface {
	// Evaluate returns true if given constraint is satisfied on
	// these labelValues.
	Evaluate(
		constraint *task.Constraint,
		labelValues LabelValues,
	) (EvaluateResult, error)
}

// EvaluateResult is an enum indicating various possible result.
// 评估结果
type EvaluateResult int

const (
	// EvaluateResultMatch indicates that given constraint fully matched
	// input labelValues.
	// 匹配
	EvaluateResultMatch EvaluateResult = iota

	// EvaluateResultMismatch indicates that given constraint mismatched
	// input labelValues.
	// 不匹配
	EvaluateResultMismatch

	// EvaluateResultNotApplicable indicates that given constraint is not
	// applicable to input values.
	// 不合适
	EvaluateResultNotApplicable
)

var (
	// ErrUnknownConstraintType is the error when unknown Constraint.Type
	// enum is processed.
	// 未知类型
	ErrUnknownConstraintType = errors.New(
		"unknown enum value for Constraint.Type")

	// ErrUnknownLabelCondition is the error when unknown
	// LabelConstraint.Condition enum is processed.
	// 未知标签条件
	ErrUnknownLabelCondition = errors.New(
		"unknown enum value for LabelConstraint.Condition")
)

// evaluator implements Evaluator by filtering out any constraint which has a
// different kind.
// 评估器，内容是标签约束类型
type evaluator task.LabelConstraint_Kind

// NewEvaluator return a new instance of evaluator which filters out constraints
// of different kind.
func NewEvaluator(kind task.LabelConstraint_Kind) Evaluator {
	return evaluator(kind)
}

// Evaluate takes given constraints and labels, and evaluate whether all parts
// in the given kind matches the input.
func (e evaluator) Evaluate(
	constraint *task.Constraint,
	labelValues LabelValues) (EvaluateResult, error) {

	switch constraint.GetType() {
	case task.Constraint_AND_CONSTRAINT:
		// 与条件
		return e.evaluateAndConstraint(constraint.GetAndConstraint(), labelValues)
	case task.Constraint_OR_CONSTRAINT:
		// 或条件
		return e.evaluateOrConstraint(constraint.GetOrConstraint(), labelValues)
	case task.Constraint_LABEL_CONSTRAINT:
		// 标签条件
		return e.evaluateLabelConstraint(constraint.GetLabelConstraint(), labelValues)
	}

	// 出错
	log.WithField("type", constraint.GetType()).
		Error(ErrUnknownConstraintType.Error())
	return EvaluateResultNotApplicable, ErrUnknownConstraintType
}

func (e evaluator) evaluateAndConstraint(
	andConstraint *task.AndConstraint,
	labelValues LabelValues,
) (EvaluateResult, error) {

	// 不合适
	result := EvaluateResultNotApplicable

	// 遍历所有约束
	for _, c := range andConstraint.GetConstraints() {
		// 计算约束结果
		subResult, err := e.Evaluate(c, labelValues)

		if err != nil {
			// 出错了
			return EvaluateResultNotApplicable, err
		}

		if subResult == EvaluateResultMismatch {
			// 不匹配
			return EvaluateResultMismatch, nil
		} else if subResult == EvaluateResultMatch {
			// 至少一个匹配了

			// If at least one constraint is relevant, we will
			// consider this is a potential match.
			result = EvaluateResultMatch
		}
	}

	return result, nil
}

func (e evaluator) evaluateOrConstraint(
	orConstraint *task.OrConstraint,
	labelValues LabelValues,
) (EvaluateResult, error) {

	result := EvaluateResultNotApplicable

	// 遍历所有约束
	for _, c := range orConstraint.GetConstraints() {
		// 计算约束结果
		subResult, err := e.Evaluate(c, labelValues)

		if err != nil {
			// 出错了
			return EvaluateResultNotApplicable, err
		}

		if subResult == EvaluateResultMatch {
			// 匹配
			return EvaluateResultMatch, nil
		} else if subResult == EvaluateResultMismatch {
			// If at least one constraint is relevant, we will
			// consider this is a potential mismatch.
			// 不匹配
			result = EvaluateResultMismatch
		}
	}

	return result, nil
}

func (e evaluator) evaluateLabelConstraint(
	labelConstraint *task.LabelConstraint,
	labelValues LabelValues,
) (EvaluateResult, error) {

	// If kind of LabelConstraint does not match, returns not applicable
	// which will not short-circuit any And/Or constraint evaluation.
	// 校验标签约束类型
	if labelConstraint.GetKind() != task.LabelConstraint_Kind(e) {
		return EvaluateResultNotApplicable, nil
	}

	// 获取需求值和规格值
	count := valueCount(labelConstraint.GetLabel(), labelValues)
	requirement := labelConstraint.GetRequirement()

	match := false

	// 判断标签条件
	switch labelConstraint.Condition {
	case task.LabelConstraint_CONDITION_LESS_THAN:
		// 小于
		match = count < requirement
	case task.LabelConstraint_CONDITION_EQUAL:
		// 相等
		match = count == requirement
	case task.LabelConstraint_CONDITION_GREATER_THAN:
		// 大于
		match = count > requirement
	default:
		// 不支持条件
		log.WithField("type", labelConstraint.Condition).
			Error(ErrUnknownLabelCondition.Error())
		return EvaluateResultNotApplicable, ErrUnknownLabelCondition
	}

	// 回包转换
	if match {
		return EvaluateResultMatch, nil
	}
	return EvaluateResultMismatch, nil
}

func valueCount(label *peloton.Label, labelValues LabelValues) uint32 {
	return labelValues[label.GetKey()][label.GetValue()]
}

// IsNonExclusiveConstraint returns true if all components of the constraint
// specification do not use a host label constraint for exclusive attribute.
// 是否独占性约束
func IsNonExclusiveConstraint(constraint *task.Constraint) bool {
	if constraint == nil {
		// 没有约束，判为true
		return true
	}

	var toEval []*task.Constraint
	switch constraint.GetType() {
	case task.Constraint_AND_CONSTRAINT:
		toEval = constraint.GetAndConstraint().GetConstraints()

	case task.Constraint_OR_CONSTRAINT:
		toEval = constraint.GetOrConstraint().GetConstraints()

	case task.Constraint_LABEL_CONSTRAINT:

		lc := constraint.GetLabelConstraint()
		if lc.GetKind() == task.LabelConstraint_HOST &&
			lc.GetLabel().GetKey() == common.PelotonExclusiveAttributeName {
			return false
		}
		return true
	}

	for _, c := range toEval {
		if !IsNonExclusiveConstraint(c) {
			// 任一个不是独占，即为不独占
			return false
		}
	}

	// 默认为独占
	return true
}
