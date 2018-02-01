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
	"time"
)

type SCIONBox struct {
	ID             uint64 `orm:"column(id);auto;pk"`
	MAC            string
	UserEmail      string
	ISD            int `orm:"default(0)"`
	AS             int `orm:"default(0)"`
	InternalIP     string
	Shipping       string
	OpenPorts      uint16 `orm:"default(0)"` // Number of free ports UDP ports starting from StartPort
	StartPort      uint16 `orm:"default(50000)"`
	UpdateRequired bool
	Created        time.Time
	Updated        time.Time
}

func FindSCIONBoxByMAC(mac string) (*SCIONBox, error) {
	v := new(SCIONBox)
	err := o.QueryTable(v).Filter("MAC", mac).RelatedSel().One(v)
	return v, err
}

func FindSCIONBoxByEMail(userEmail string) (*SCIONBox, error) {
	v := new(SCIONBox)
	err := o.QueryTable(v).Filter("UserEmail", userEmail).RelatedSel().One(v)
	return v, err
}

func FindSCIONBoxByIAint(isd int, As int) (*SCIONBox, error) {
	v := new(SCIONBox)
	err := o.QueryTable(v).Filter("ISD", isd).Filter("AS", As).RelatedSel().One(v)
	return v, err
}

func (sb *SCIONBox) Insert() error {
	sb.Created = time.Now().UTC()
	sb.Updated = time.Now().UTC()
	_, err := o.Insert(sb)
	return err
}

func (sb *SCIONBox) Update() error {
	sb.Updated = time.Now().UTC()
	_, err := o.Update(sb)
	return err
}

func (sb *SCIONBox) Remove() error {
	_, err := o.Delete(sb)
	return err
}

type ISDLocation struct {
	Id        uint64 `orm:"column(id);auto;pk"`
	ISD       int
	Country   string
	Continent string
}

func (il *ISDLocation) Insert() error {
	_, err := o.Insert(il)
	return err
}

func (il *ISDLocation) Update() error {
	_, err := o.Update(il)
	return err
}

func FindISDbyID(id int) (*ISDLocation, error) {
	v := new(ISDLocation)
	err := o.QueryTable(v).Filter("ISD", id).RelatedSel().One(v)
	return v, err
}

func FindISDbyCountry(country string) (*ISDLocation, error) {
	v := new(ISDLocation)
	err := o.QueryTable(v).Filter("Country", country).RelatedSel().One(v)
	return v, err
}

func FindISDbyContinent(continent string) (*ISDLocation, error) {
	v := new(ISDLocation)
	err := o.QueryTable(v).Filter("Continent", continent).RelatedSel().One(v)
	return v, err
}

// Find Potential Neighbors for the Box
func FindPotentialNeighbors(isd int) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	v, err := GetAllAPsByISD(isd)
	return v, err
}

// Find All Active Attachment Points in an ISD
func GetAllAPsByISD(isd int) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	w, err := GetAllAPs()
	if err != nil {
		return nil, err
	}
	for _, ap := range w {
		if ap.ISD == isd {
			if ap.Status == ACTIVE {
				v = append(v, *ap)
			}
		}
	}
	return v, nil
}

func FindSCIONLabAsesByISD(isd int) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("ISD", isd).RelatedSel().All(&v)
	return v, err
}
