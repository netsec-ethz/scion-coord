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
	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion/go/lib/addr"
	"log"
	"time"
)

type As struct {
	Id      uint64   `orm:"column(id);auto;pk"`
	Isd     int      `orm:"index"`
	As      int      `orm:"index"`
	Core    bool     `orm:"default(false)"`
	Account *Account `orm:"rel(fk);index"`
	Credits int64
	Created time.Time
}

func FindCoreASesByIsd(isd int) ([]As, error) {
	var ases []As
	_, err := o.QueryTable("as").Filter("Isd", isd).Filter("Core", true).All(&ases)
	return ases, err
}

func FindASesByIsd(isd int) ([]As, error) {
	var ases []As
	_, err := o.QueryTable("as").Filter("Isd", isd).All(&ases)
	return ases, err
}

func FindAsByIsdAs(isdas string) (*As, error) {
	ia, err := addr.IAFromString(isdas)
	if err != nil {
		return nil, err
	}
	as := new(As)
	err = o.QueryTable(as).Filter("Isd", ia.I).Filter("As", ia.A).RelatedSel().One(as)
	return as, err
}

type ConnectionWithCredits struct {
	ISD int
	AS int
	CreditBalance int64
	Bandwidth uint64
	IsOutgoing bool

}


func (as *As) ListConnections() ([]ConnectionWithCredits, error) {
	var connections []ConnectionWithCredits
	var isdas = fmt.Sprint("%v-%v", as.Isd, as.As)

	// Outgoing ones (this AS paid for)
	var outGoings []ConnRequest
	_, err := o.QueryTable("conn_request").Filter("status", APPROVED).Filter("request_i_a", isdas).All(&outGoings)
	if err != nil {
		return connections, err
	}
	for _, v := range outGoings {
		var tmp,_ = addr.IAFromString(v.RespondIA)
		connections = append(connections, ConnectionWithCredits{
			ISD: tmp.I,
			AS: tmp.A,
			Bandwidth: v.Bandwidth,
			CreditBalance: BandwidthToCredits(v.Bandwidth),
			IsOutgoing: true,
		})
	}

	// Ingoing ones (this AS get Credits for)
	var inGoings []ConnRequest
	_, err = o.QueryTable("conn_request").Filter("status", APPROVED).Filter("respond_i_a", isdas).All(&inGoings)
	if err != nil {
		return connections, err
	}
	for _, v := range inGoings {
		var tmp,_ = addr.IAFromString(v.RespondIA)
		connections = append(connections, ConnectionWithCredits{
			ISD: tmp.I,
			AS: tmp.A,
			Bandwidth: v.Bandwidth,
			CreditBalance: BandwidthToCredits(v.Bandwidth),
			IsOutgoing: false,
		})
	}

	return connections, err
}

func (as *As) deleteAs() error {
	_, err := o.Delete(as)
	return err
}

func (as *As) Insert() error {
	// First check whether this AS already exists, duplicates are not allowed.
	existing_as := new(As)
		// should always return with orm.ErrNoRows
	err := o.QueryTable(as).Filter("Isd", as.Isd).Filter("As", as.As).RelatedSel().One(existing_as)
	if err == nil {
		log.Printf("ISD-AS (%v-%v) already exists, will not be re-inserted", as.Isd, as.As)
		return fmt.Errorf("ISD-AS (%v-%v) already exists, will not be re-inserted", as.Isd, as.As)
	} else if err != orm.ErrNoRows { // some other error occurred during lookup
		return err
	}
	as.Credits = StartCredits()
	_, err = o.Insert(as)
	return err
}

func (as *As) UpdateCurrency(Credits int64) error {
	as.Credits += Credits
	_, err := o.Update(as, "Credits")
	return err
}

func (as *As) String() string {
	return fmt.Sprintf("%v-%v", as.Isd, as.As)
}
