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

package models

import (
	"time"

	"github.com/uber/peloton/.gen/peloton/private/resmgr"
	"github.com/uber/peloton/pkg/placement/plugins"
)

// Offer is the interface that represents a Host Offer in v0, or
// a Host Lease in v1alpha API.
type Offer interface {
    // 主机插件
	plugins.Host

	// Returns the ID of the offer (Offer ID or Lease ID)
	// id
	ID() string

	// Returns the hostname of the offer.
	// 主机名
	Hostname() string

	// Returns the agent ID that this offer is for.
	// ？？？？
	AgentID() string

	// Returns the available port ranges for this offer.
	// 可用端口列表
	AvailablePortRanges() map[*PortRange]struct{}
}

// Task is the interface that represents a resource manager task. This
// is meant to wrap around the resource manager protos so we can keep the same logic
// for v0 and v1alpha API.
// 任务
type Task interface {
    // 任务插件
	plugins.Task

	// This ID is either the mesos task id or the k8s pod name.
	OrchestrationID() string

	// Returns true if the task is past its placement deadline.
	// 是否超时
	IsPastDeadline(time.Time) bool

	// Increments the number of placement rounds that we have
	// tried to place this task for.
	// 重试一次
	IncRounds()

	// IsPastMaxRounds returns true if the task has been through
	// too many placement rounds.
	// 是否重试过多
	IsPastMaxRounds() bool

	// Returns true if the task is ready for host reservation.
	// 是否准备好了
	IsReadyForHostReservation() bool

	// Returns true if the task should run on revocable resources.
	// 可取消资源？
	IsRevocable() bool

	// Sets the matching offer for this task.
	// 布局
	SetPlacement(Offer)

	// Returns the offer that this tasked was matched with.
	// 获取布局
	GetPlacement() Offer

	// Returns the reason for the placement failure.
	// 布局失败原因
	GetPlacementFailure() string

	// Returns the resource manager v0 task.
	// NOTE: This was done to get the host reservation feature working. We
	// should figure out a way to avoid having to do this.
	// 资源管理器任务
	GetResmgrTaskV0() *resmgr.Task
}

// ToPluginTasks transforms an array of tasks into an array of placement
// strategy plugin tasks.
// 转换
func ToPluginTasks(tasks []Task) []plugins.Task {
	result := make([]plugins.Task, len(tasks))
	for i, t := range tasks {
		result[i] = t
	}
	return result
}
