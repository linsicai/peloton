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

package common

import (
	"go.uber.org/yarpc/yarpcerrors"
)

// IsTransientError returns true if the error was transient and the overall
// operation should be retried.
// 是否短暂错误
func IsTransientError(err error) bool {
	if yarpcerrors.IsAlreadyExists(err) {
	    // 已存在
		return true
	}
	if yarpcerrors.IsAborted(err) {
	    // 取消
		return true
	}

	if yarpcerrors.IsUnavailable(err) {
	    // 不可用
		return true
	}

	if yarpcerrors.IsDeadlineExceeded(err) {
	    // 超时
		return true
	}

	return false
}
