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

package watchevent

import (
	"fmt"
	"sync"

	halphapb "github.com/uber/peloton/.gen/peloton/api/v1alpha/host"
	"github.com/uber/peloton/.gen/peloton/private/eventstream"

	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/uber/peloton/pkg/hostmgr/metrics"
	"go.uber.org/yarpc/yarpcerrors"
)

// StopSignal is an event sent through event client Signal channel
// indicating a stop event for the specific watcher.
// 停止信号
type StopSignal int

const (
	// StopSignalUnknown indicates a unspecified StopSignal.
	// 未知的停止信号
	StopSignalUnknown StopSignal = iota

	// StopSignalCancel indicates the watch is cancelled by the user.
	// 用户取消停止信号
	StopSignalCancel

	// StopSignalOverflow indicates the watch is aborted due to event
	// overflow.
	// 过载停止信号
	StopSignalOverflow
)

// String returns a user-friendly name for the specific StopSignal
// 停止信号类型转字符串
func (s StopSignal) String() string {
	switch s {
	case StopSignalCancel:
		return "cancel"
	case StopSignalOverflow:
		return "overflow"
	default:
		return "unknown"
	}
}

// WatchProcessor interface is a central controller which handles watch
// client lifecycle, and task / job event fan-out.
// watch 接口
type WatchProcessor interface {
	// NewEventClient creates a new watch client for mesos task event changes.
	// Returns the watch id and a new instance of EventClient.
	// 创建事件客户端
	NewEventClient(topic Topic) (string, *EventClient, error)

	// StopEventClients stops all the event clients on leadership change.
	// 停止所有事件
	StopEventClients()

	// StopEventClient stops a event watch client. Returns "not-found" error
	// if the corresponding watch client is not found.
	// 停止单个事件
	StopEventClient(watchID string) error

	// NotifyEventChange receives mesos task event, and notifies all the clients
	// which are interested in the event.
	// 通知事件变更
	NotifyEventChange(event interface{})
}

// watchProcessor is an implementation of WatchProcessor interface.
type watchProcessor struct {
	sync.Mutex // 锁

	// 配置
	bufferSize int // 客户端缓存大小
	maxClient  int // 最大客户端数目

	// 映射表
	eventClients      map[string]*EventClient   // 客户端映射表
	topicEventClients map[Topic]map[string]bool // 主题事件映射表

	metrics *metrics.Metrics // 指标
}

// Topic define the event object type processor supported
type Topic string

// List of topic currently supported by watch processor
const (
	EventStream Topic = "eventstream" // 事件流主题
	HostSummary Topic = "hostSummary" // 主机概述
	INVALID     Topic = ""            // 非法主题
)

var processor *watchProcessor
var onceInitWatchProcessor sync.Once

// EventClient represents a client which interested in task event changes.
type EventClient struct {
	Input  chan interface{} // 事件通道
	Signal chan StopSignal  // 停止信号
}

// newWatchProcessor should only be used in unit tests.
// Call InitWatchProcessor for regular case use.
func NewWatchProcessor(
	cfg Config,
	watchEventMetric *metrics.Metrics,
) *watchProcessor {
	// 配置归一化
	cfg.normalize()

	return &watchProcessor{
		bufferSize:        cfg.BufferSize,
		maxClient:         cfg.MaxClient,
		eventClients:      make(map[string]*EventClient),
		topicEventClients: make(map[Topic]map[string]bool),
		metrics:           watchEventMetric,
	}
}

// InitWatchProcessor initializes WatchProcessor singleton.
// 初始化watch 处理器
func InitWatchProcessor(
	cfg Config,
	watchEventMetric *metrics.Metrics,
) {
	onceInitWatchProcessor.Do(func() {
		processor = NewWatchProcessor(cfg, watchEventMetric)
	})
}

// GetWatchProcessor returns WatchProcessor singleton.
func GetWatchProcessor() WatchProcessor {
	return processor
}

// NewWatchID creates a new watch id UUID string for the specific
// watch client
// 创建watch id
func NewWatchID(topic Topic) string {
	// 主题 + uuid
	return fmt.Sprintf("%s_%s_%s", topic, "watch", uuid.New())
}

// Retrieve the topic from the event object received during NotifyEventChange
// 获取事件类型
func GetTopicFromTheEvent(event interface{}) Topic {
	switch event.(type) {
	case *eventstream.Event:
		return EventStream
	case *halphapb.HostSummary:
		return HostSummary
	default:
		return INVALID
	}
}

// Map the string receive from input to a right topic
// 主题转事件类型
func GetTopicFromInput(topic string) Topic {
	switch topic {
	case "eventstream":
		return EventStream
	case "hostSummary":
		return HostSummary
	default:
		return INVALID
	}
}

// NewEventClient creates a new watch client for task event changes.
// Returns the watch id and a new instance of EventClient.
// 创建新的事件客户端
func (p *watchProcessor) NewEventClient(topic Topic) (string, *EventClient, error) {
	// 锁处理
	p.Lock()
	defer p.Unlock()

	// 客户端阈值校验
	if len(p.eventClients) >= p.maxClient {
		return "", nil, yarpcerrors.ResourceExhaustedErrorf("max client reached")
	}

	// 创建新的watch id
	watchID := NewWatchID(topic)

	// 创建watch client
	p.eventClients[watchID] = &EventClient{
		Input: make(chan interface{}, p.bufferSize),
		// Make buffer size 1 so that sender is not blocked when sending
		// the Signal
		Signal: make(chan StopSignal, 1),
	}

	// 创建主题目事件映射表
	if p.topicEventClients[topic] == nil {
		p.topicEventClients[topic] = make(map[string]bool)
	}
	p.topicEventClients[topic][watchID] = true

	// 返回
	log.WithField("watch_id", watchID).Info("task watch client created")
	return watchID, p.eventClients[watchID], nil
}

// StopEventClients stops all the event clients on host manager leader change
func (p *watchProcessor) StopEventClients() {
	p.Lock()
	defer p.Unlock()

	// 遍历所有客户端，发送停止信号
	for watchID := range p.eventClients {
		p.stopEventClient(watchID, StopSignalCancel)
	}
}

// StopEventClient stops a event watch client. Returns "not-found" error
// if the corresponding watch client is not found.
func (p *watchProcessor) StopEventClient(watchID string) error {
	p.Lock()
	defer p.Unlock()

	return p.stopEventClient(watchID, StopSignalCancel)
}

// 注意无锁
func (p *watchProcessor) stopEventClient(
	watchID string,
	Signal StopSignal,
) error {
	// 校验watch id
	c, ok := p.eventClients[watchID]
	if !ok {
		return yarpcerrors.NotFoundErrorf(
			"watch_id %s not exist for task watch client", watchID)
	}

	// 日志
	log.WithFields(log.Fields{
		"watch_id": watchID,
		"signal":   Signal,
	}).Info("stopping  watch client")

	// 向客户端发送停止信号
	c.Signal <- Signal

	// 清理映射表
	delete(p.eventClients, watchID)
	for _, watchIdMap := range p.topicEventClients {
		delete(watchIdMap, watchID)
	}

	return nil
}

// NotifyTaskChange receives mesos task update event, and notifies all the clients
// which are interested in the task update event.
// 通知事件变更
func (p *watchProcessor) NotifyEventChange(event interface{}) {
	sw := p.metrics.WatchProcessorLockDuration.Start()

	p.Lock()
	defer p.Unlock()

	sw.Stop()

	// get topic  of the event
	// 找主题
	topic := GetTopicFromTheEvent(event)

	if topic == "" {
		// 没有主题，日志一下即可
		log.WithFields(log.Fields{
			"event": event,
			"topic": string(topic),
		}).Warn("topic  not supported, please register topic to the watch processor")

	} else {
		// 遍历主题下所有客户端，发送事件
		for watchID := range p.topicEventClients[topic] {
			select {
			case p.eventClients[watchID].Input <- event:
				// 发送时间
			default:
				// 发送事件失败，停止客户端
				log.WithField("watch_id", watchID).
					Warn("event overflow for task watch client")
				p.stopEventClient(watchID, StopSignalOverflow)
			}
		}
	}
}
