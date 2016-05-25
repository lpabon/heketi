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
	"testing"

	"github.com/heketi/tests"
)

func TestEntryStateString(t *testing.T) {
	var e EntryState

	tests.Assert(t, "Unknown" == fmt.Sprintf("%v", e))

	e = EntryStateOnline
	tests.Assert(t, "Online" == fmt.Sprintf("%v", e))

	e = EntryStateOffline
	tests.Assert(t, "Offline" == fmt.Sprintf("%v", e))

	e = EntryStateFailed
	tests.Assert(t, "Failed" == fmt.Sprintf("%v", e))
}
