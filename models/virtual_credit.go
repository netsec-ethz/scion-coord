// Copyright 2016 ETH Zurich
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

import (
	"fmt"

	"github.com/netsec-ethz/scion/go/lib/addr"
)

// The start credits for every new AS
func StartCredits() int64 {
	// 100 credits = 1000 MegaBits/s worth.
	// Maybe will be later in- or decreased
	return 100
}

// Converts a bandwidth into credits (10 MegaBits/s=1 Credit)
func BandwidthToCredits(bandwidthInKilobits uint64) int64 {
	return int64(bandwidthInKilobits) / 10000
}

// Converts credits into bandwidth (1 Credit=10 MegaBits/s)
func CreditsToBandwidthInBandwidthInKilobits(credits uint64) int64 {
	return int64(credits) * 10000
}

// Stores the connection info from an AS to another AS
type ConnectionWithCredits struct {
	ISD           int    // ISD of the other AS
	AS            int    // the other AS
	CreditBalance int64  // How much credits the connection costs / yields
	Bandwidth     uint64 // The bandwidth in kb/s
	IsOutgoing    bool   // false = the other AS has to pay for, true = the other AS gets credits for
	Timestamp     string // The creation time of the connection
}

// Look for all connection to and from this AS and calculates the necessary credits for it
func (as *As) ListConnections() ([]ConnectionWithCredits, error) {
	var connections []ConnectionWithCredits
	var isdas = fmt.Sprintf("%v-%v", as.Isd, as.As)

	// Outgoing ones (this AS paid for)
	var outGoings []ConnRequest
	_, err := o.QueryTable("conn_request").Filter("status", APPROVED).Filter("request_i_a", isdas).All(&outGoings)
	if err != nil {
		return connections, err
	}
	for _, v := range outGoings {
		var targetAS, _ = addr.IAFromString(v.RespondIA)
		connections = append(connections, ConnectionWithCredits{
			ISD:           targetAS.I,
			AS:            targetAS.A,
			Bandwidth:     v.Bandwidth,
			CreditBalance: BandwidthToCredits(v.Bandwidth),
			IsOutgoing:    true,
			Timestamp:     v.Timestamp,
		})
	}

	// incoming ones (this AS get Credits for)
	var inComings []ConnRequest
	_, err = o.QueryTable("conn_request").Filter("status", APPROVED).Filter("respond_i_a", isdas).All(&inComings)
	if err != nil {
		return connections, err
	}
	for _, v := range inComings {
		var sourceAS, _ = addr.IAFromString(v.RequestIA)
		connections = append(connections, ConnectionWithCredits{
			ISD:           sourceAS.I,
			AS:            sourceAS.A,
			Bandwidth:     v.Bandwidth,
			CreditBalance: BandwidthToCredits(v.Bandwidth),
			IsOutgoing:    false,
			Timestamp:     v.Timestamp,
		})
	}

	return connections, err
}
