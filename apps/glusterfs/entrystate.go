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

import (
	"fmt"
	"strings"
)

type EntryState string

const (
	EntryStateUnknown EntryState = ""
	EntryStateOnline  EntryState = "online"
	EntryStateOffline EntryState = "offline"
	EntryStateFailed  EntryState = "failed"
)

func NewEntryState(s string) (EntryState, error) {
	newstate := EntryState(strings.ToLower(s))

	switch newstate {
	case EntryStateOnline:
		fallthrough
	case EntryStateOffline:
		fallthrough
	case EntryStateFailed:
		return newstate, nil
	default:
		return "", fmt.Errorf("Unknown state requested: %v", s)
	}
}

func SetEntryState(s EntryState) (EntryState, error) {
	return NewEntryState(string(s))
}
