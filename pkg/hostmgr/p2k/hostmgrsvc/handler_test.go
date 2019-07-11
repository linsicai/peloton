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

package hostmgrsvc

import (
	"fmt"
	"strings"
	"testing"

	pbhost "github.com/uber/peloton/.gen/peloton/api/v1alpha/host"
	"github.com/uber/peloton/.gen/peloton/api/v1alpha/peloton"
	pbpod "github.com/uber/peloton/.gen/peloton/api/v1alpha/pod"
	hostmgr "github.com/uber/peloton/.gen/peloton/private/hostmgr/v1alpha"
	"github.com/uber/peloton/.gen/peloton/private/hostmgr/v1alpha/svc"
	hostcache_mocks "github.com/uber/peloton/pkg/hostmgr/p2k/hostcache/mocks"
	plugins_mocks "github.com/uber/peloton/pkg/hostmgr/p2k/plugins/mocks"

	"github.com/golang/mock/gomock"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	"github.com/uber-go/tally"
	"golang.org/x/net/context"
)

const (
	_perHostCPU = 10.0
	_perHostMem = 20.0
)

var (
	rootCtx    = context.Background()
	_testJobID = uuid.New()
	_podIDFmt  = _testJobID + "-%d"
)

// generate launchable pod specs
func generateLaunchablePods(numPods int) []*hostmgr.LaunchablePod {
	var pods []*hostmgr.LaunchablePod
	for i := 0; i < numPods; i++ {
		pods = append(pods, &hostmgr.LaunchablePod{
			PodId: &peloton.PodID{Value: fmt.Sprintf(_podIDFmt, i)},
			Spec: &pbpod.PodSpec{
				Containers: []*pbpod.ContainerSpec{
					{
						Name: uuid.New(),
						Resource: &pbpod.ResourceSpec{
							CpuLimit:   1.0,
							MemLimitMb: 100.0,
						},
						Ports: []*pbpod.PortSpec{
							{
								Name:  "port",
								Value: 80,
							},
						},
						Image: "nginx",
					},
				},
			},
		})
	}
	return pods
}

// HostMgrHandlerTestSuite tests the v1alpha service handler for hostmgr
type HostMgrHandlerTestSuite struct {
	suite.Suite

	ctrl      *gomock.Controller
	testScope tally.TestScope
	hostCache *hostcache_mocks.MockHostCache
	plugin    *plugins_mocks.MockPlugin
	handler   *ServiceHandler
}

// SetupTest sets up tests in HostMgrHandlerTestSuite
func (suite *HostMgrHandlerTestSuite) SetupTest() {
	suite.ctrl = gomock.NewController(suite.T())

	suite.testScope = tally.NewTestScope("", map[string]string{})

	suite.plugin = plugins_mocks.NewMockPlugin(suite.ctrl)
	suite.hostCache = hostcache_mocks.NewMockHostCache(suite.ctrl)

	suite.handler = &ServiceHandler{
		plugin:    suite.plugin,
		hostCache: suite.hostCache,
	}
}

// TearDownTest tears down tests in HostMgrHandlerTestSuite
func (suite *HostMgrHandlerTestSuite) TearDownTest() {
	log.Debug("tearing down")
}

// TestAcquireHosts tests AcquireHosts API
func (suite *HostMgrHandlerTestSuite) TestAcquireHosts() {
	defer suite.ctrl.Finish()

	testTable := map[string]struct {
		filter       *hostmgr.HostFilter
		filterResult map[string]uint32
		leases       []*hostmgr.HostLease
		errMsg       string
	}{
		"acquire-success": {
			filter: &hostmgr.HostFilter{
				ResourceConstraint: &hostmgr.ResourceConstraint{
					Minimum: &pbpod.ResourceSpec{
						CpuLimit:   _perHostCPU,
						MemLimitMb: _perHostMem,
					},
				},
			},
			filterResult: map[string]uint32{
				strings.ToLower("HOST_FILTER_MATCH"): 1,
			},
			leases: []*hostmgr.HostLease{
				{
					HostSummary: &pbhost.HostSummary{
						Hostname: "test",
					},
					LeaseId: &hostmgr.LeaseID{
						Value: uuid.New(),
					},
				},
			},
		},
	}
	for ttName, tt := range testTable {
		req := &svc.AcquireHostsRequest{
			Filter: tt.filter,
		}
		suite.hostCache.EXPECT().
			AcquireLeases(tt.filter).
			Return(tt.leases, tt.filterResult)

		resp, err := suite.handler.AcquireHosts(rootCtx, req)
		if tt.errMsg != "" {
			suite.Equal(tt.errMsg, err.Error(), "test case %s", ttName)
			continue
		}
		suite.NoError(err, "test case %s", ttName)
		suite.Equal(tt.leases, resp.GetHosts())
		suite.Equal(tt.filterResult, resp.GetFilterResultCounts())
	}
}

// TestLaunchPods tests LaunchPods API
func (suite *HostMgrHandlerTestSuite) TestLaunchPods() {
	defer suite.ctrl.Finish()

	testTable := map[string]struct {
		errMsg         string
		launchablePods []*hostmgr.LaunchablePod
		leaseID        *hostmgr.LeaseID
		hostname       string
	}{
		"launch-pods-success": {
			launchablePods: generateLaunchablePods(10),
			hostname:       "host-name",
			leaseID:        &hostmgr.LeaseID{Value: uuid.New()},
		},
	}
	for ttName, tt := range testTable {
		req := &svc.LaunchPodsRequest{
			LeaseId:  tt.leaseID,
			Hostname: tt.hostname,
			Pods:     tt.launchablePods,
		}
		suite.hostCache.EXPECT().
			CompleteLease(tt.hostname, tt.leaseID.GetValue(), gomock.Any()).
			Return(nil)
		for _, pod := range tt.launchablePods {
			suite.plugin.EXPECT().
				LaunchPod(
					pod.GetSpec(),
					pod.GetPodId().GetValue(),
					tt.hostname,
				).Return(nil)
		}

		resp, err := suite.handler.LaunchPods(rootCtx, req)
		if tt.errMsg != "" {
			suite.Equal(tt.errMsg, err.Error(), "test case %s", ttName)
			continue
		}
		suite.NoError(err, "test case %s", ttName)
		suite.Equal(&svc.LaunchPodsResponse{}, resp)
	}
}

// TestHostManagerTestSuite runs the HostMgrHandlerTestSuite
func TestHostManagerTestSuite(t *testing.T) {
	suite.Run(t, new(HostMgrHandlerTestSuite))
}