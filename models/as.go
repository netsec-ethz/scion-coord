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
	"strconv"
	"strings"
	"time"
)

type As struct {
	IsdAs   string   `orm:"index;pk"`
	Isd     uint64   `orm:"index"`
	Core    bool     `orm:"default(false)"`
	Account *Account `orm:"rel(fk);index"`
	Created time.Time
}

func FindCoreASesByIsd(isd uint64) ([]As, error) {
	var ases []As
	_, err := o.QueryTable("as").Filter("Isd", isd).Filter("Core", true).All(&ases)
	return ases, err
}

func FindCoreAsByIsdAs(isd uint64, isd_as string) (*As, error) {
	as := new(As)
	err := o.QueryTable("as").Filter("Isd", isd).Filter("IsdAs", isd_as).Filter("Core", true).RelatedSel().One(as)
	return as, err
}

func FindAsByIsdAs(isd_as string) (*As, error) {
	as := new(As)
	err := o.QueryTable(as).Filter("IsdAs", isd_as).RelatedSel().One(as)
	return as, err
}

func AllASes() ([]As, error) {
	var ASes []As
	_, err := o.QueryTable("as").All(&ASes)
	return ASes, err
}

func (as *As) deleteAs() error {
	_, err := o.Delete(as)
	return err
}

func (as *As) Insert() error {
	_, err := o.Insert(as)
	return err
}

func IsdAsToIsd(isdas string) (uint64, error) {
	return strconv.ParseUint(strings.Split(isdas, "-")[0], 10, 64)
}

func FindNumAsInIsd(isd uint64) (int64, error) {
	return o.QueryTable("as").Filter("Isd", isd).Count()
}
