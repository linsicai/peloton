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

// Nomination represents the set of callbacks to handle leadership
// election
// 提名接口
type Nomination interface {
	// GainedLeadershipCallback is the callback when the current node
	// becomes the leader
	// 获取领导者回调
	GainedLeadershipCallback() error

	// HasGainedLeadership returns true iff once GainedLeadershipCallback
	// completes
	// 是否获取领导者成功
	HasGainedLeadership() bool

	// ShutDownCallback is the callback to shut down gracefully if
	// possible
	// 关闭回调
	ShutDownCallback() error

	// LostLeadershipCallback is the callback when the leader lost
	// leadership
	/// 失去领导者回调
	LostLeadershipCallback() error

	// GetID returns the host:master_port of the node running for
	// leadership (i.e. the ID)
	// 获取id
	GetID() string
}

// Candidate is an interface representing both a candidate campaigning
// to become a leader, and the observer that watches state changes in
// leadership to be notified about changes
// 候选人接口
type Candidate interface {
	// 是否领导者
	IsLeader() bool

	// 开始与结束
	Start() error
	Stop() error

	// 辞职接口
	Resign()
}
