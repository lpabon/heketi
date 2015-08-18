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

package glusterfs

type EntryState int

const (
	ENTRY_STATE_CREATING EntryState = iota
	ENTRY_STATE_READY
	ENTRY_STATE_DELETING
	ENTRY_STATE_MODIFYING
)

type EntryStateMachine struct {
	State   EntryState
	Counter int
}

func NewEntryStateMachine() *EntryStateMachine {
	return &EntryStateMachine{}
}

func (e *EntryStateMachine) GetState() EntryState {
	return e.State
}

func (e *EntryStateMachine) SetState(s EntryState) error {

	switch e.State {

	case ENTRY_STATE_CREATING:
		panic("Cannot move into creating mode")

	case ENTRY_STATE_DELETING:
		if s != ENTRY_STATE_READY {
			return ErrEntryBusy
		}

	case ENTRY_STATE_MODIFYING:
		switch s {
		case ENTRY_STATE_DELETING:
			return ErrEntryBusy
		case ENTRY_STATE_MODIFYING:
			e.Counter++
		case ENTRY_STATE_READY:
			e.Counter--
			if e.Counter <= 0 {
				e.Counter = 0
				e.State = ENTRY_STATE_READY
			}
		}

	case ENTRY_STATE_READY:
		e.State = s
	}

	return nil
}
