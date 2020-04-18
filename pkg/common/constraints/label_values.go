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
	"strconv"

	mesos "github.com/uber/peloton/.gen/mesos/v1"
	"github.com/uber/peloton/pkg/common"

	log "github.com/sirupsen/logrus"
)

const (
	_precision = 6  // 精度
	_bitsize   = 64 // 位长
)

// LabelValues tracks how many times a value presents for a given label key.
// First level key is label key, second level key is label value.
// This is the subject of constraint evaluation process.
type LabelValues map[string]map[string]uint32

// Merge merges additional label values into current label values.
// Values of the same label will be merged as well.
func (lv LabelValues) Merge(additionalLV LabelValues) {
	for label, origin := range additionalLV {
		if origin == nil {
			// pass 空值
			continue
		}

		// 复制值
		copy := make(map[string]uint32)
		for value, count := range origin {
			copy[value] = count
		}

		if _, ok := lv[label]; !ok {
			// 新key
			lv[label] = copy
			continue
		}
		for v, c := range copy {
			if current, ok := lv[label][v]; !ok {
				// 新的
				lv[label][v] = c
			} else {
				// 累加
				lv[label][v] = current + c
			}
		}
	}
}

// GetHostLabelValues returns label counts for a host and its attributes,
// which can be used to evaluate a constraint.
// NOTE: `hostname` is added unconditionally, to make sure hostname based
// constraints can be done regardless of attribute configuration.
// 获取主机标签值
func GetHostLabelValues(
	hostname string,
	attributes []*mesos.Attribute) LabelValues {

	result := make(map[string]map[string]uint32)
	result[common.HostNameKey] = map[string]uint32{hostname: 1}

OUTER:
	// 遍历属性
	for _, attr := range attributes {
		values := []string{}

		// 区别属性类型
		key := attr.GetName()
		switch attr.GetType() {
		case mesos.Value_TEXT:
			// 文本
			values = append(values, attr.GetText().GetValue())
		case mesos.Value_SCALAR:
			// 数值
			value := strconv.FormatFloat(
				attr.GetScalar().GetValue(),
				'f',
				_precision,
				_bitsize)
			values = append(values, value)
		case mesos.Value_SET:
			// 列表
			for _, value := range attr.GetSet().GetItem() {
				values = append(values, value)
			}
		default:
			// 不支持
			// TODO: Add support for range attributes.
			log.WithFields(log.Fields{
				"key":  key,
				"type": attr.GetType(),
			}).Warn("Attribute type is not supported yet")
			continue OUTER
		}

		// 结果初始化
		if _, ok := result[key]; !ok {
			result[key] = make(map[string]uint32)
		}

		// 标签值
		for _, value := range vlues {
			result[key][value] = result[key][value] + 1
		}
	}
	return result
}
