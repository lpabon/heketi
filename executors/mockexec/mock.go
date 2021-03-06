//
// Copyright (c) 2015 The heketi Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package mockexec

import (
	"github.com/heketi/heketi/executors"
)

type MockExecutor struct {
	// These functions can be overwritten for testing
	MockPeerProbe      func(exec_host, newnode string) error
	MockPeerDetach     func(exec_host, newnode string) error
	MockDeviceSetup    func(host, device, vgid string) (*executors.DeviceInfo, error)
	MockDeviceTeardown func(host, device, vgid string) error
}

func NewMockExecutor() *MockExecutor {
	m := &MockExecutor{}

	m.MockPeerProbe = func(exec_host, newnode string) error {
		return nil
	}

	m.MockPeerDetach = func(exec_host, newnode string) error {
		return nil
	}

	m.MockDeviceSetup = func(host, device, vgid string) (*executors.DeviceInfo, error) {
		d := &executors.DeviceInfo{}
		d.Size = 10 * 1024 * 1024 // Size in KB
		return d, nil
	}

	m.MockDeviceTeardown = func(host, device, vgid string) error {
		return nil
	}

	return m
}

func (m *MockExecutor) PeerProbe(exec_host, newnode string) error {
	return m.MockPeerProbe(exec_host, newnode)
}

func (m *MockExecutor) PeerDetach(exec_host, newnode string) error {
	return m.MockPeerDetach(exec_host, newnode)
}

func (m *MockExecutor) DeviceSetup(host, device, vgid string) (*executors.DeviceInfo, error) {
	return m.MockDeviceSetup(host, device, vgid)
}

func (m *MockExecutor) DeviceTeardown(host, device, vgid string) error {
	return m.MockDeviceTeardown(host, device, vgid)
}
