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
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"

	"gopkg.in/validator.v2"
	"gopkg.in/yaml.v2"
)

// PelotonSecretsConfig will be used to interpret secrets mounted
// for Peloton service
// 密钥配置
type PelotonSecretsConfig struct {
	CassandraUsername string `yaml:"peloton_cassandra_username"`
	CassandraPassword string `yaml:"peloton_cassandra_password"`
}

// ValidationError is the returned when a configuration fails to pass validation
// 验证错误
type ValidationError struct {
	errorMap validator.ErrorMap
}

// ErrForField returns the validation error for the given field
func (e ValidationError) ErrForField(name string) error {
	return e.errorMap[name]
}

// Error returns the error string from a ValidationError
func (e ValidationError) Error() string {
	var w bytes.Buffer

	fmt.Fprintf(&w, "validation failed")
	for f, err := range e.errorMap {
		fmt.Fprintf(&w, "   %s: %v\n", f, err)
	}

	return w.String()
}

// Parse loads the given configFiles in order, merges them together, and parse into given
// config interface.
func Parse(config interface{}, configFiles ...string) error {
	if len(configFiles) == 0 {
	    // 无配置文件
		return errors.New("no files to load")
	}

	for _, fname := range configFiles {
	    // 读取文件
		data, err := ioutil.ReadFile(fname)
		if err != nil {
			return err
		}

        // 反序列化
		if err := yaml.Unmarshal(data, config); err != nil {
			return err
		}
	}

	// Validate on the merged config at the end.
	// 校验配置
	if err := validator.Validate(config); err != nil {
		return ValidationError{
			errorMap: err.(validator.ErrorMap),
		}
	}
	return nil
}
