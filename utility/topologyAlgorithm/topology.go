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

import (
	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/models"
)

// performance score thresholds
const (
	bw1  = 0.05
	bw2  = 0.1
	bw3  = 0.5
	rtt1 = 10
	rtt2 = 50
	rtt3 = 100
)

// number of neighbors chosen for each AS
const (
	chosenNeighbors uint16 = 3
)

// maximal number of neighbors chosen for each AS
const (
	maxNeighbors = 6
)

type Neighbor struct {
	ISD int
	AS  int
	IP  string
	BW  float64
	RTT float64
}

// Choose up to 3 of the best potential neighbors in the array
func ChooseNeighbors(potentialneighbors []Neighbor, freePorts uint16) []Neighbor {
	var neighbors []Neighbor
	var counter uint16 = 0
	// compute number of new neighbors that will be chosen
	newNeighbors := chosenNeighbors
	if freePorts < newNeighbors {
		newNeighbors = freePorts
	}
	for counter < newNeighbors {
		if len(potentialneighbors) == 0 {
			break
		}
		counter++
		bnb, index := chooseBestNeighbor(potentialneighbors)
		neighbors = append(neighbors, bnb)
		// remove the chosen neighbor from the list of potential neighbors
		potentialneighbors = removeNeighbor(potentialneighbors, index)
	}
	return neighbors
}

// Choose the best Neighbor from a list of neighbors
// Best neighbor is the neighbor with lowest PF
// If PF are the same the neighbor with lower degree is chosen
func chooseBestNeighbor(potentialneighbors []Neighbor) (Neighbor, int) {
	var bestNb = potentialneighbors[0]
	var index = 0
	for i, nb := range potentialneighbors {
		if bestNb.getDegree() >= maxNeighbors {
			bestNb = nb
			index = i
		}
		if nb.getDegree() >= maxNeighbors {
			continue
		}
		if bestNb.getPF() > nb.getPF() {
			sbNb, err := models.FindSCIONBoxByIAint(nb.ISD, nb.AS)
			if err != nil {
				if err == orm.ErrNoRows {
					bestNb = nb
					index = i
				}
			} else {
				var nbFreePorts = sbNb.OpenPorts - nb.getDegree()
				if nbFreePorts > 0 {
					bestNb = nb
					index = i
				}
			}
		}
		if bestNb.getPF() == nb.getPF() {
			// same PF score, AS with lower degree is chosen
			sbNb, err := models.FindSCIONBoxByIAint(nb.ISD, nb.AS)
			if err != nil {
				if err == orm.ErrNoRows {
					if bestNb.getDegree() > nb.getDegree() {
						bestNb = nb
						index = i
					}
				}
			} else {
				if bestNb.getDegree() > nb.getDegree() {
					var nbFreePorts = sbNb.OpenPorts - nb.getDegree()
					if nbFreePorts > 0 {
						bestNb = nb
						index = i
					}
				}
			}
		}
	}
	return bestNb, index
}

// Compute the Performance Class of a neighbors connection
// Four PF classes 1: best ,.. 4: worst
// Returns 5 if not classable (Error has occurred)
func (nb Neighbor) getPF() int {
	if nb.BW == -1 || nb.RTT == -1 {
		return 5
	}
	var bw = nb.BW
	var rtt = nb.RTT
	if bw > bw3 {
		return 4
	}
	if bw3 >= bw && bw > bw2 {
		if rtt <= rtt3 {
			return 3
		} else {
			return 4
		}
	}
	if bw2 >= bw && bw > bw1 {
		if rtt > rtt3 {
			return 4
		}
		if rtt3 >= rtt && rtt > rtt2 {
			return 3
		}
		if rtt2 >= rtt {
			return 2
		}
	}
	if bw1 >= bw {
		if rtt > rtt3 {
			return 4
		}
		if rtt3 >= rtt && rtt > rtt2 {
			return 3
		}
		if rtt2 >= rtt && rtt > rtt1 {
			return 2
		}
		if rtt1 >= rtt {
			return 1
		}
	}
	return 5
}

// Get the number of neighbors from the database
// If an error occurs return 9999
func (nb Neighbor) getDegree() uint16 {
	dbEntry, err := models.FindSCIONLabASByIAInt(nb.ISD, nb.AS)
	if err != nil {
		return 9999
	}
	cns, err := dbEntry.GetConnectionInfo()
	if err != nil {
		return 9999
	}
	return uint16(len(cns))
}

// Remove element at index i from array
func removeNeighbor(neighbors []Neighbor, i int) []Neighbor {
	neighbors[len(neighbors)-1], neighbors[i] = neighbors[i], neighbors[len(neighbors)-1]
	return neighbors[:len(neighbors)-1]
}
