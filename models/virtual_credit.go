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

func StartCredits() int64 {
	return 100
}

func BandwidthToCredits(bandwidthInKilobits uint64) int64 {
	return int64(bandwidthInKilobits) / 10000
}

func CreditsToBandwidthInBandwidthInKilobits(credits uint64) int64 {
	return int64(credits) * 10000
}

type ConnectionWithCredits struct {
	ISD           int
	AS            int
	CreditBalance int64
	Bandwidth     uint64
	IsOutgoing    bool
	Timestamp     string
}

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
		var tmp, _ = addr.IAFromString(v.RespondIA)
		connections = append(connections, ConnectionWithCredits{
			ISD:           tmp.I,
			AS:            tmp.A,
			Bandwidth:     v.Bandwidth,
			CreditBalance: BandwidthToCredits(v.Bandwidth),
			IsOutgoing:    true,
			Timestamp:     v.Timestamp,
		})
	}

	// Ingoing ones (this AS get Credits for)
	var inGoings []ConnRequest
	_, err = o.QueryTable("conn_request").Filter("status", APPROVED).Filter("respond_i_a", isdas).All(&inGoings)
	if err != nil {
		return connections, err
	}
	for _, v := range inGoings {
		var tmp, _ = addr.IAFromString(v.RequestIA)
		connections = append(connections, ConnectionWithCredits{
			ISD:           tmp.I,
			AS:            tmp.A,
			Bandwidth:     v.Bandwidth,
			CreditBalance: BandwidthToCredits(v.Bandwidth),
			IsOutgoing:    false,
			Timestamp:     v.Timestamp,
		})
	}

	return connections, err
}
