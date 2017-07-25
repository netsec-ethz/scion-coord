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

type ScionLabServer struct {
	Id               uint64 `orm:"column(id);auto;pk"`
	IA               string `orm:"unique"` // ISD-AS in which the server is located
	IP               string // IP address of the machine
	LastAssignedPort int    // the last given out port number
}

func (sls *ScionLabServer) Insert() error {
	_, err := o.Insert(sls)
	return err
}

func (sls *ScionLabServer) Update() error {
	_, err := o.Update(sls)
	return err
}

func FindScionLabServer(ia string) (*ScionLabServer, error) {
	s := new(ScionLabServer)
	err := o.QueryTable(s).Filter("IA", ia).One(s)
	return s, err
}

type ScionLabVM struct {
	Id           uint64 `orm:"column(id);auto;pk"`
	UserEmail    string `orm:"unique"`        // Email address of the Owning user
	IP           string `orm:"unique"`        // IP address of the ScionLab VM
	IA           *As    `orm:"rel(fk);index"` // The AS belonging to the VM
	RemoteIA     string // the SCIONLab AS it connects to
	RemoteIAPort int    // port number of the remote SCIONLab AS being connected to
	Activated    bool
}

func FindScionLabVMByUserEmail(email string) (*ScionLabVM, error) {
	v := new(ScionLabVM)
	err := o.QueryTable(v).Filter("UserEmail", email).RelatedSel().One(v)
	return v, err
}

func FindScionLabVMByIPAndIA(ip, ia string) (*ScionLabVM, error) {
	v := new(ScionLabVM)
	err := o.QueryTable(v).Filter("IP", ip).Filter("RemoteIA", ia).RelatedSel().One(v)
	return v, err
}

func FindScionLabVMsByRemoteIA(remoteIA string) ([]ScionLabVM, error) {
	var v []ScionLabVM
	_, err := o.QueryTable("scion_lab_v_m").Filter("RemoteIA", remoteIA).Filter("Activated", false).RelatedSel().All(&v)
	return v, err
}

func (svm *ScionLabVM) Insert() error {
	_, err := o.Insert(svm)
	return err
}

func (svm *ScionLabVM) Update() error {
	_, err := o.Update(svm)
	return err
}
