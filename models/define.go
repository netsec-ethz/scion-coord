// Copyright 2017 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

// States in which a SCIONLabAS or connection can be.
const (
	Inactive = iota // 0
	Active
	Create
	Update
	Remove
	Removed
)

// Link types
const (
	Parent = iota // 0
	Child
	Core
	Peer
)

func LinkTypeString(linkType uint8) string {
	switch linkType {
	case Parent:
		return "PARENT"
	case Child:
		return "CHILD"
	case Core:
		return "CORE"
	case Peer:
		return "PEER"
	default:
		return ""
	}
}

// Types of SCIONLabASes
const (
	Infrastructure = iota // 0
	VM
	Dedicated
	Box
)
