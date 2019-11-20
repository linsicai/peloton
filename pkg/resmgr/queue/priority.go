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

package queue

import (
	"errors"
	"fmt"
	"sync"

	"github.com/uber/peloton/.gen/peloton/private/resmgrsvc"

	log "github.com/sirupsen/logrus"
)

// PriorityQueue is FIFO queue which remove the highest priority task item entered first in the queue
type PriorityQueue struct {
	sync.RWMutex
	list MultiLevelList
}

// NewPriorityQueue intializes the fifo queue and returns the pointer
func NewPriorityQueue(limit int64) *PriorityQueue {
	fq := PriorityQueue{
		list: NewMultiLevelList("list", limit),
	}

	return &fq
}

// Enqueue queues a gang (task list gang) based on its priority into FIFO queue
func (f *PriorityQueue) Enqueue(gang *resmgrsvc.Gang) error {
    // 加锁
	f.Lock()
	defer f.Unlock()

    // 参数校验
	if (gang == nil) || (len(gang.Tasks) == 0) {
		return errors.New("enqueue of empty list")
	}

    // 获取优先级
	tasks := gang.GetTasks()
	priority := tasks[0].Priority

    // 入队列
	return f.list.Push(int(priority), gang)
}

// Dequeue dequeues the gang (task list gang) based on the priority and order
// they came into the queue
func (f *PriorityQueue) Dequeue() (*resmgrsvc.Gang, error) {
	// TODO: optimize the write lock here with potential read lock
	// 加锁
	f.Lock()
	defer f.Unlock()

    // 获取最高优先级
	highestPriority := f.list.GetHighestLevel()

    // 尝试取一条记录
	item, err := f.list.Pop(highestPriority)
	if err != nil {
	    // 取数据出错了

		// TODO: Need to add test case for this case
		// 尝试取其他优先级队列
		for highestPriority != f.list.GetHighestLevel() {
			highestPriority = f.list.GetHighestLevel()
			item, err = f.list.Pop(highestPriority)
			if err == nil {
				break
			}
		}

        // 真的出错了
		if err != nil {
			return nil, err
		}
	}

    // 队列空
	if item == nil {
		return nil, errors.New("dequeue failed")
	}

    // 返回数据
	res := item.(*resmgrsvc.Gang)
	return res, nil
}

// Peek peeks the limit number of gangs based on the priority and order
// they came into the queue.
// It will return an `ErrorQueueEmpty` if there is no gangs in the queue
func (f *PriorityQueue) Peek(limit uint32) ([]*resmgrsvc.Gang, error) {
	// TODO: optimize the write lock here with potential read lock
	// 加锁
	f.Lock()
	defer f.Unlock()

	var items []*resmgrsvc.Gang

    // 定位最高优先级
	priority := f.list.GetHighestLevel()
	itemsLeft := int(limit)

	// start at the highest priority
	// keep going down until priority 0 or until limit is satisfied
	// 循环获取
	for {
		if priority < 0 {
			// min priority is 0; we are done
			// 没有优先级队列可以获取了
			break
		}

		if itemsLeft == 0 {
			// we are done
			// 获取够了
			break
		}

		itemsByPriority, err := f.list.PeekItems(priority, itemsLeft)
		if err != nil {
		    // 获取出错
			if _, ok := err.(ErrorQueueEmpty); ok {
				// no items for priority, continue to the next one
				// 空队列换个优先级
				priority--
				continue
			}
			return items, fmt.Errorf("peek failed err: %s", err)
		}

		// 保存数据
		gangs := toGang(itemsByPriority)
		items = append(items, gangs...)

        // 更新下一个优先级和剩余额度
		priority--
		itemsLeft = itemsLeft - len(itemsByPriority)
	}

	if len(items) == 0 {
		return items, ErrorQueueEmpty("peek failed, queue is empty")
	}

	return items, nil
}

func toGang(items []interface{}) []*resmgrsvc.Gang {
	var gangs []*resmgrsvc.Gang
	for _, item := range items {
		res := item.(*resmgrsvc.Gang)
		gangs = append(gangs, res)
	}
	return gangs
}

// Remove removes the item from the queue
func (f *PriorityQueue) Remove(gang *resmgrsvc.Gang) error {
    // 加锁
	f.Lock()
	defer f.Unlock()

    // 参数判断
	if gang == nil || len(gang.Tasks) <= 0 {
		return errors.New("removal of empty list")
	}

    // 找优先级
	firstItem := gang.Tasks[0]
	priority := firstItem.Priority

    // 日志
	log.WithFields(log.Fields{
		"item ":    firstItem,
		"priority": priority,
	}).Debug("Trying to remove")

    // 移出队列
	return f.list.Remove(int(priority), gang)
}

// Len returns the length of the queue for specified priority
func (f *PriorityQueue) Len(priority int) int {
	return f.list.Len(priority)
}

// Size returns the number of elements in the PriorityQueue
func (f *PriorityQueue) Size() int {
	return f.list.Size()
}
