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

package leader

import (
	"github.com/uber-go/tally"
)

// 选举指标
type electionMetrics struct {
	Start tally.Counter
	Stop  tally.Counter

	Resigned         tally.Counter // 辞职
	LostLeadership   tally.Counter // 丢失领导者
	GainedLeadership tally.Counter // 获得领导者
	IsLeader         tally.Gauge   // 是领导者

	Running tally.Gauge   // 运行
	Error   tally.Counter // 错误
}

// 观察指标
type observerMetrics struct {
	Start tally.Counter // 启动
	Stop  tally.Counter // 停止

	LeaderChanged tally.Counter // 领导者改变
	Running       tally.Gauge   // 运行
	Error         tally.Counter // 错误
}

//TODO(rcharles) replace tag hostname with instanceNumber
// 创建选举者指标
func newElectionMetrics(scope tally.Scope, hostname string) electionMetrics {
	s := scope.Tagged(map[string]string{"hostname": hostname})

	return electionMetrics{
		Start:            s.Counter("start"),
		Stop:             s.Counter("stop"),
		Resigned:         s.Counter("resigned"),
		LostLeadership:   s.Counter("lost_leadership"),
		GainedLeadership: s.Counter("gained_leadership"),
		IsLeader:         s.Gauge("is_leader"),
		Running:          s.Gauge("running"),
		Error:            s.Counter("error"),
	}
}

// 观察者指标
func newObserverMetrics(scope tally.Scope, role string) observerMetrics {
	s := scope.Tagged(map[string]string{"role": role})

	return observerMetrics{
		Start:         s.Counter("start"),
		Stop:          s.Counter("stop"),
		LeaderChanged: s.Counter("leader_changed"),
		Running:       s.Gauge("running"),
		Error:         s.Counter("error"),
	}
}
