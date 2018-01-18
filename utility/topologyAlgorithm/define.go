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

package topologyAlgorithm

// performance score thresholds
const (
	BW1  = 0.05
	BW2  = 0.1
	BW3  = 0.5
	RTT1 = 10
	RTT2 = 50
	RTT3 = 100
)
// number of neighbors chosen for each AS
const(
	CHOSEN_NEIGHBORS = 3
)
// maximal number of neighbors chosen for each AS
const(
	MAX_NEIGHBORS = 6
)