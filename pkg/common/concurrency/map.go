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

package concurrency

import (
	"context"
	"sync"
)

// Mapper maps inputs into outputs.
// 映射
type Mapper interface {
	Map(ctx context.Context, input interface{}) (output interface{}, err error)
}

// MapperFunc is an adaptor to allow the use of ordinary functions as Mappers.
type MapperFunc func(ctx context.Context, input interface{}) (output interface{}, err error)

// Map calls f.
// 调用函数
func (f MapperFunc) Map(ctx context.Context, input interface{}) (output interface{}, err error) {
	return f(ctx, input)
}

// map 结果
type mapResult struct {
	output interface{}
	err    error
}

// Map applies m.Map to inputs using numWorkers goroutines. Collects the
// outputs or stops early if error is encountered.
func Map(
	ctx context.Context,
	m Mapper,
	inputs []interface{},
	numWorkers int,
) (outputs []interface{}, err error) {

	// Ensure all channel sends/recvs have a release valve if we encounter
	// an early error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	inputc := make(chan interface{})
	resultc := make(chan *mapResult)

    // 并发
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case i, ok := <-inputc:
				    // 拉取输入

					if !ok {
					    // 结束了
						return
					}

					// 做map
					o, err := m.Map(ctx, i)
					// 取结果
					select {
					case resultc <- &mapResult{o, err}:
					case <-ctx.Done():
						return
					}
				case <-ctx.Done():
				    // 结束
					return
				}
			}
		}()
	}

	go func() {
	    // 遍历输入，放到通道中
		for _, i := range inputs {
			select {
			case inputc <- i:
			case <-ctx.Done():
				return
			}
		}
		// 输入结束
		close(inputc)  // Signal to workers there are no more inputs.

        // 等待所有map 结束
		wg.Wait()      // Wait for workers to finish in-progress work.

        // 可以取结果了
		close(resultc) // Signal to consumer that work is finished.
	}()

	// 取结果
	for {
		select {
		case r, ok := <-resultc:
		    // 读到结果

			if !ok {
			    // 没有结束
				return outputs, nil
			}
			if r.err != nil {
			    // 失败了
				return nil, r.err
			}
			if r.output != nil {
			    // 累计结果
				outputs = append(outputs, r.output)
			}
		case <-ctx.Done():
		    // 上下文结束了
			return nil, ctx.Err()
		}
	}
}
