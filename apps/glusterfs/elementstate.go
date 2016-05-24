//
// Copyright (c) 2016 The heketi Authors
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

package glusterfs

type ElementState int

const (
	ELEMENT_STATE_UNKNOWN ElementState = iota
	ELEMENT_STATE_ONLINE
	ELEMENT_STATE_OFFLINE
	ELEMENT_STATE_FAILED
)

func (e ElementState) String() string {
	switch e {
	case ELEMENT_STATE_ONLINE:
		return "Online"
	case ELEMENT_STATE_OFFLINE:
		return "Offline"
	case ELEMENT_STATE_FAILED:
		return "Failed"
	}

	return "Unknown"
}
