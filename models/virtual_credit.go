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

import (
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
)

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
	IsOutgoing    bool   // false = the other AS has to pay, true = the other AS gets credits
	Timestamp     string // The creation time of the connection
}

// Look for all connection to and from this AS and calculates the necessary credits for it
func (asInfo *ASInfo) ListConnections() ([]ConnectionWithCredits, error) {
	var connections []ConnectionWithCredits
	isdas := utility.IAString(asInfo.ISD, asInfo.ASID)

	// Outgoing ones (this AS pays for)
	var outGoings []ConnRequest
	_, err := o.QueryTable(new(ConnRequest)).Filter("Status", APPROVED).Filter("RequestIA",
		isdas).All(&outGoings)
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

	// Incoming ones (this AS gets Credits for)
	var inComings []ConnRequest
	_, err = o.QueryTable(new(ConnRequest)).Filter("Status", APPROVED).Filter("RespondIA",
		isdas).All(&inComings)
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

// Changes the Credits the AS has. CreditsDiff can be negative to subtract and be positive to add
// Credits
func (asInfo *ASInfo) UpdateCurrency(CreditsDiff int64) error {
	as, err := FindSCIONLabASByASInfo(*asInfo)
	if err != nil {
		return err
	}
	as.Credits += CreditsDiff
	return as.Update()
}
