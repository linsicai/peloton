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

package apiserver

// Config contains APIServer specific configuration
type Config struct {
	// HTTP port which API Server is listening on
	// http 端口
	HTTPPort int `yaml:"http_port"`

	// gRPC port which API Server is listening on
	// grpc 端口
	GRPCPort int `yaml:"grpc_port"`
}
