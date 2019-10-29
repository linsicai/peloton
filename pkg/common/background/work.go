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

package background

import (
	"sync"
	"time"

	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/uber-go/atomic"
)

const (
	_stopRetryInterval = 1 * time.Millisecond
)

var (
    // 空名字
	errEmptyName     = errors.New("background work name cannot be empty")
	// 重复队列
	errDuplicateName = errors.New("duplicate background work name")
)

// Work refers to a piece of background work which needs to happen
// periodically.
type Work struct {
	Name         string // 名字
	Func         func(*atomic.Bool) // 函数

	Period       time.Duration // 触发间隔
	InitialDelay time.Duration // 初始化延时
}

// Manager allows multiple background Works to be registered and
// started/stopped together.
type Manager interface {
	// Start starts all registered background works.
	Start()
	// Stop starts all registered background works.
	Stop()

	// RegisterWork registers a background work against the Manager
	// 注册
	RegisterWorks(works ...Work) error
}

// manager implements Manager interface.
type manager struct {
	runners map[string]*runner
}

// NewManager creates a new instance of Manager with registered background works.
func NewManager() Manager {
	return &manager{
		runners: make(map[string]*runner),
	}
}

// RegisterWorks registers background works against the Manager
func (r *manager) RegisterWorks(works ...Work) error {
	for _, work := range works {
		if work.Name == "" {
		    // 没有名字
			return errEmptyName
		}

		if _, ok := r.runners[work.Name]; ok {
		    // 已经存在
			return errDuplicateName
		}

        // 入映射表
		r.runners[work.Name] = &runner{
			work:     work,
			stopChan: make(chan struct{}, 1),
		}
	}
	return nil
}

// Start all registered works.
func (r *manager) Start() {
    // 启动
	for _, runner := range r.runners {
		runner.start()
	}
}

// Stop all registered runners.
func (r *manager) Stop() {
    // 停止
	for _, runner := range r.runners {
		runner.stop()
	}
}

// 运行类
type runner struct {
    // 锁
	sync.Mutex

	work Work

	running  atomic.Bool

    // 停止信号
	stopChan chan struct{}
}

func (r *runner) start() {
	log.WithField("name", r.work.Name).Info("Starting Background work.")

    // 加锁
	r.Lock()
	defer r.Unlock()

    // 校验运行状态
	if r.running.Swap(true) {
	    // 已经运行中
		log.WithField("name", r.work.Name).
			WithField("interval_secs", r.work.Period.Seconds()).
			Info("Background work is already running, no-op.")
		return
	}

	go func() {
	    // 退出后设置停止
		defer r.running.Store(false)

		// Non empty initial delay
		// 预热
		if r.work.InitialDelay.Nanoseconds() > 0 {
		    // 日志
			log.WithField("name", r.work.Name).
				WithField("initial_delay", r.work.InitialDelay).
				Info("Initial delay for background work")

            // 初始化定时器
			initialTimer := time.NewTimer(r.work.InitialDelay)
			select {
			case <-r.stopChan:
				log.Info("Periodic reconcile stopped before first run.")
				// 退出
				return
			case <-initialTimer.C:
				log.Debug("Initial delay passed")
			}

            // 做事情
			r.work.Func(&r.running)
		}

      
        // 创建运行计时器
		ticker := time.NewTicker(r.work.Period)
		defer ticker.Stop()
		for {
			select {
			case <-r.stopChan:
				log.WithField("name", r.work.Name).
					Info("Background work stopped.")
				// 停止了
				return
			case t := <-ticker.C:
			    // 定时触发
				log.WithField("tick", t).
					WithField("name", r.work.Name).
					Debug("Background work triggered.")
				r.work.Func(&r.running)
			}
		}
	}()
}

func (r *runner) stop() {
	log.WithField("name", r.work.Name).Info("Stopping Background work.")

    // 校验是否运行
	if !r.running.Load() {
		log.WithField("name", r.work.Name).
			Warn("Background work is not running, no-op.")
		return
	}

    // 加锁
	r.Lock()
	defer r.Unlock()

    // 发送停止信号
	r.stopChan <- struct{}{}

	// TODO: Make this non-blocking.
	// 阻塞等待停止
	for r.running.Load() {
		time.Sleep(_stopRetryInterval)
	}
	log.WithField("name", r.work.Name).Info("Background work stop confirmed.")
}
