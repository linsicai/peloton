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

package config

import (
	"fmt"
	"path"
	"strings"

	"github.com/uber/peloton/.gen/mesos/v1"

	"go.uber.org/thriftrw/ptr"
)

const (
    // 分隔符
	ThermosExecutorDelimiter     = ","
	ThermosExecutorIDPlaceholder = "PLACEHOLDER"
)

// ThermosExecutorConfig wraps the config for thermos executor
type ThermosExecutorConfig struct {
	// The following themros executor related flags should have identical
	// behavior as the one with Aurora Scheduler:
	// http://aurora.apache.org/documentation/latest/reference/scheduler-configuration/
	//
	// path:
	//   Path to the thermos executor entry point.
	// resources:
	//   A comma separated list of additional
	//   resources to copy into the sandbox.Note: if thermos_executor_path
	//   is not the thermos_executor.pex file itself, this must include it.
	// flags:
	//   Extra arguments to be passed to the thermos executor
	// cpu:
	//   The number of CPU cores to allocate for each instance of the executor.
	// ram:
	//   The amount of RAM in MB to allocate for each instance of the executor.
	Path      string  `yaml:"path"` // 路径
	Resources string  `yaml:"resources"` // 资源
	Flags     string  `yaml:"flags"` ：// 标记
	CPU       float64 `yaml:"cpu"` // CPU
	RAM       int64   `yaml:"ram"` // 内存
}

// Validate validates ThermosExecutorConfig
// 验证
func (c ThermosExecutorConfig) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("thermos_executor_path not provided")
	}
	return nil
}

// NewThermosCommandInfo creates Mesos CommandInfo for thermos executor
// similar to Aurora's behavior. Reference:
// https://github.com/apache/aurora/blob/master/src/main/java/org/apache/aurora/scheduler/configuration/executor/ExecutorModule.java#L120
func (c ThermosExecutorConfig) NewThermosCommandInfo() *mesos_v1.CommandInfo {
    // 资源列表
	resourcesToFetch := []string{c.Path}
	if c.Resources != "" {
		resourcesToFetch = append(
			resourcesToFetch,
			strings.Split(c.Resources, ThermosExecutorDelimiter)...,
		)
	}

    // 拼mesos 资源uri
	var mesosUris []*mesos_v1.CommandInfo_URI
	for _, r := range resourcesToFetch {
		mesosUris = append(mesosUris, &mesos_v1.CommandInfo_URI{
			Value:      ptr.String(r),
			Executable: ptr.Bool(true),
		})
	}

    // 路径与标记
	var b strings.Builder
	b.WriteString("${MESOS_SANDBOX=.}/")
	b.WriteString(path.Base(c.Path))
	b.WriteString(" ")
	b.WriteString(c.Flags)
	mesosValue := strings.TrimSpace(b.String())

    // 真实命令
	return &mesos_v1.CommandInfo{
		Uris:  mesosUris,
		Shell: ptr.Bool(true),
		Value: ptr.String(mesosValue),
	}
}

// NewThermosExecutorInfo creates Mesos ExecutorInfo for thermos executor.
func (c ThermosExecutorConfig) NewThermosExecutorInfo(executorData []byte) *mesos_v1.ExecutorInfo {
	var r []*mesos_v1.Resource
	if c.CPU > 0 {
	    // cpu
		r = append(r, &mesos_v1.Resource{
			Type: mesos_v1.Value_SCALAR.Enum(),
			Name: ptr.String("cpus"),
			Scalar: &mesos_v1.Value_Scalar{
				Value: ptr.Float64(c.CPU),
			},
		})
	}
	if c.RAM > 0 {
	    // 内存
		r = append(r, &mesos_v1.Resource{
			Type: mesos_v1.Value_SCALAR.Enum(),
			Name: ptr.String("mem"),
			Scalar: &mesos_v1.Value_Scalar{
				Value: ptr.Float64(float64(c.RAM)),
			},
		})
	}

	// ExecutorId will be filled by hostmgr during task launch.
	// 执行信息
	return &mesos_v1.ExecutorInfo{
		Type: mesos_v1.ExecutorInfo_CUSTOM.Enum(),
		ExecutorId: &mesos_v1.ExecutorID{
			Value: ptr.String(ThermosExecutorIDPlaceholder),
		},
		Resources: r,
		Data:      executorData,
	}
}
