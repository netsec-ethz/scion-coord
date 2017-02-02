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
	"log"
	"strconv"
	"strings"
	"time"
)

type As struct {
	Id      uint64   `orm:"column(id);auto;pk"`
	Isd     uint64   `orm:"index"`
	As      uint64   `orm:"index"`
	Core    bool     `orm:"default(false)"`
	Account *Account `orm:"rel(fk);index"`
	Created time.Time
}

func FindCoreASesByIsd(isd uint64) ([]As, error) {
	var ases []As
	_, err := o.QueryTable("as").Filter("Isd", isd).Filter("Core", true).All(&ases)
	return ases, err
}

func FindAsByIsdAs(isdas string) (*As, error) {
	isd_nr, err := IsdAsToIsd(isdas)
	if err != nil {
		// logging is already done in the called function
		return nil, err
	}
	as_nr, err := IsdAsToAs(isdas)
	if err != nil {
		// logging is already done in the called function
		return nil, err
	}
	as := new(As)
	err = o.QueryTable(as).Filter("Isd", isd_nr).Filter("As", as_nr).RelatedSel().One(as)
	return as, err
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
	_, err = o.Insert(as)
	return err
}

func (as *As) ToStr() string {
	return fmt.Sprintf("%v-%v", as.Isd, as.As)
}

func IsdAsToIsd(isdas string) (uint64, error) {
	isd, err := strconv.ParseUint(strings.Split(isdas, "-")[0], 10, 64)
	if err != nil {
		log.Printf("Error extracting ISD from ISD-AS: %v, %v", isdas, err)
		return 0, err
	}
	return isd, nil
}

func IsdAsToAs(isdas string) (uint64, error) {
	as, err := strconv.ParseUint(strings.Split(isdas, "-")[1], 10, 64)
	if err != nil {
		log.Printf("Error extracting AS from ISD-AS: %v, %v", isdas, err)
		return 0, err
	}
	return as, nil
}

func ParseIsdAs(isdas string) (uint64, uint64, error) {
	isd_nr, err := IsdAsToIsd(isdas)
	if err != nil {
		return 0, 0, nil
	}
	as_nr, err := IsdAsToAs(isdas)
	if err != nil {
		return 0, 0, nil
	}
	return isd_nr, as_nr, nil
}
