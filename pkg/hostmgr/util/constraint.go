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

package util

import (
	mesos "github.com/uber/peloton/.gen/mesos/v1"
	"github.com/uber/peloton/.gen/peloton/api/v0/task"
	"github.com/uber/peloton/.gen/peloton/private/hostmgr/hostsvc"
	"github.com/uber/peloton/pkg/common/constraints"

	log "github.com/sirupsen/logrus"
)

func MatchSchedulingConstraint(
	hostname string,
	additionalLV constraints.LabelValues,
	attributes []*mesos.Attribute,
	constraint *task.Constraint,
	evaluator constraints.Evaluator,
) hostsvc.HostFilterResult {
	// If constraints don't specify an exclusive host, then reject
	// hosts that are designated as exclusive.
	if constraints.IsNonExclusiveConstraint(constraint) && HasExclusiveAttribute(attributes) {
		// 有独占属性，且为独占性约束
		log.WithField("hostname", hostname).Debug("Skipped exclusive host")

		// 返回不匹配
		return hostsvc.HostFilterResult_MISMATCH_CONSTRAINTS
	}

	// 约束为空，判为匹配
	if constraint == nil {
		// No scheduling constraint, we have a match.
		return hostsvc.HostFilterResult_MATCH
	}

	// 计算主机标签+附加标签
	lv := constraints.GetHostLabelValues(
		hostname,
		attributes,
	)
	lv.Merge(additionalLV)

	// 评估器计算
	result, err := evaluator.Evaluate(constraint, lv)
	if err != nil {
		log.WithError(err).
			Error("Error when evaluating input constraint")
		return hostsvc.HostFilterResult_MISMATCH_CONSTRAINTS
	}

	switch result {
	case constraints.EvaluateResultMatch:
		fallthrough
	case constraints.EvaluateResultNotApplicable:
		log.WithFields(log.Fields{
			"values":     lv,
			"hostname":   hostname,
			"constraint": constraint,
		}).Debug("Attributes match constraint")
	default:
		log.WithFields(log.Fields{
			"values":     lv,
			"hostname":   hostname,
			"constraint": constraint,
		}).Debug("Attributes do not match constraint")
		// 不匹配
		return hostsvc.HostFilterResult_MISMATCH_CONSTRAINTS
	}

	// 匹配
	return hostsvc.HostFilterResult_MATCH
}
