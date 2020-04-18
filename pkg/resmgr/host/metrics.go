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

package host

import "github.com/uber-go/tally"

// Metrics is a placeholder for all metrics in host.
// 排水指标
type Metrics struct {
	HostDrainSuccess tally.Counter // 排水成功数
	HostDrainFail    tally.Counter // 排水失败数
}

// NewMetrics returns a new instance of host.Metrics.
func NewMetrics(scope tally.Scope) *Metrics {
	hostSuccessScope := scope.Tagged(map[string]string{"type": "success"})
	hostFailScope := scope.Tagged(map[string]string{"type": "fail"})

	return &Metrics{
		HostDrainSuccess: hostSuccessScope.Counter("host_drain"),
		HostDrainFail:    hostFailScope.Counter("host_drain"),
	}
}
