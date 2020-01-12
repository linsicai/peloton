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

// PortRange represents a modifiable closed-open port range [Begin:End[
// used when assigning ports to tasks.
// 端口范围
type PortRange struct {
	Begin uint64
	End   uint64
}

// NewPortRange creates a new modifiable port range from a Mesos value range.
// Both the begin and end ports are part of the port range.
func NewPortRange(begin, end uint64) *PortRange {
	return &PortRange{
		Begin: begin,
		End:   end + 1,
	}
}

// NumPorts returns the number of available ports in the range.
// 端口数量
func (portRange *PortRange) NumPorts() uint64 {
	return portRange.End - portRange.Begin
}

// TakePorts will take the number of ports from the range or as many as
// available if more ports are requested than are in the range.
func (portRange *PortRange) TakePorts(numPorts uint64) []uint64 {
	// Try to select ports in a random fashion to avoid ports conflict.
	ports := make([]uint64, 0, numPorts)

    // 计算停止位
	stop := portRange.Begin + numPorts
	if numPorts >= portRange.NumPorts() {
		stop = portRange.End
	}

    // 计算拿走的端口
	for i := portRange.Begin; i < stop; i++ {
		ports = append(ports, i)
	}

    // 更新开始位置
	portRange.Begin = stop
	return ports
}

// AssignPorts selects available ports from the offer and returns them.
func AssignPorts(
	offer Offer,
	tasks []Task,
) []uint64 {
    // 可用端口范围
	availablePortRanges := offer.AvailablePortRanges()

    // 遍历任务
	var selectedPorts []uint64
	for _, taskEntity := range tasks {
	    // 已分配端口数
		assignedPorts := uint64(0)

		// 获取任务需要端口数
		neededPorts := taskEntity.GetPlacementNeeds().Ports

		// 遍历可用端口范围
		depletedRanges := []*PortRange{}
		for portRange := range availablePortRanges {
			// 尝试分配
			ports := portRange.TakePorts(neededPorts - assignedPorts)
			assignedPorts += uint64(len(ports))
			selectedPorts = append(selectedPorts, ports...)
			if portRange.NumPorts() == 0 {
			    // 如果范围用完了，记录下，后续删除
				depletedRanges = append(depletedRanges, portRange)
			}
			if assignedPorts >= neededPorts {
				// 已分配完
				break
			}
		}

        // 删除可用端口范围
		for _, portRange := range depletedRanges {
			delete(availablePortRanges, portRange)
		}
	}

	return selectedPorts
}
