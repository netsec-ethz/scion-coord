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
	"fmt"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/utility"
)

type SCIONLabServer struct {
	Id                uint64 `orm:"column(id);auto;pk"`
	IA                string `orm:"unique"` // ISD-AS in which the server is located
	IP                string `orm:"unique"` // IP address of the machine
	LastAssignedPort  int    // the last given out port number
	UserPort          int    `orm:"unique"` // Port of border router on user end
	VPNIP             string // IP address of the machine inside the VPN
	VPNLastAssignedIP string // the last given out IP address for the VPN
}

func (sls *SCIONLabServer) Insert() error {
	_, err := o.Insert(sls)
	return err
}

func (sls *SCIONLabServer) Update() error {
	_, err := o.Update(sls)
	return err
}

func FindSCIONLabServer(ia string) (*SCIONLabServer, error) {
	s := new(SCIONLabServer)
	err := o.QueryTable(s).Filter("IA", ia).One(s)
	return s, err
}

func GetAllSCIONLabServerIAs() ([]*SCIONLabServer, error) {
	var servers []*SCIONLabServer
	_, err := o.QueryTable(new(SCIONLabServer)).All(&servers)
	return servers, err
}

type SCIONLabVM struct {
	Id           uint64 `orm:"column(id);auto;pk"`
	UserEmail    string // Email address of the Owning user
	IP           string // IP address of the SCIONLab VM (does not have to be unique in case of VPN setup)
	IA           *As    `orm:"rel(fk);index"` // The AS belonging to the VM
	IsVPN        bool   // is this VM connected via the VPN
	RemoteIA     string // the SCIONLab AS it connects to
	RemoteIAPort int    // port number of the remote SCIONLab AS being connected to
	RemoteBR     string `orm:"unique"`     // the name of the remote border router for this AS
	Status       uint8  `orm:"default(0)"` // Status of the VM (i.e Active, Create, Update, Remove)
}

func FindSCIONLabVMByServerIAAndUserEmail(ia, email string) (*SCIONLabVM, error) {
	v := new(SCIONLabVM)
	err := o.QueryTable(v).Filter("UserEmail", email).Filter("RemoteIA", ia).RelatedSel().One(v)
	return v, err
}

func FindSCIONLabVMsByUserEmail(email string) ([]*SCIONLabVM, error) {
	var v []*SCIONLabVM
	_, err := o.QueryTable(new(SCIONLabVM)).Filter("UserEmail", email).RelatedSel().All(&v)
	return v, err
}

func FindSCIONLabVMByIPAndRemoteIA(ip, ia string) (*SCIONLabVM, error) {
	v := new(SCIONLabVM)
	err := o.QueryTable(v).Filter("IP", ip).Filter("RemoteIA", ia).RelatedSel().One(v)
	return v, err
}

func FindSCIONLabVMsByRemoteIA(remoteIA string) ([]*SCIONLabVM, error) {
	var v []*SCIONLabVM
	_, err := o.QueryTable(new(SCIONLabVM)).Filter("RemoteIA", remoteIA).RelatedSel().All(&v)
	return v, err
}

func (svm *SCIONLabVM) Insert() error {
	_, err := o.Insert(svm)
	return err
}

func (svm *SCIONLabVM) Update() error {
	_, err := o.Update(svm)
	return err
}

// Inserts or updates database entry for given server configuration
func InsertUpdateSLS(serverConfig config.SCIONLabServerConfig) error {
	sls, err := FindSCIONLabServer(serverConfig.IA)
	if err != nil {
		if err == orm.ErrNoRows { // Server does not exist
			newSLS := SCIONLabServer{
				IA:                serverConfig.IA,
				IP:                serverConfig.IP,
				UserPort:          serverConfig.UserPort,
				LastAssignedPort:  serverConfig.ServerStartPort,
				VPNIP:             serverConfig.VPNIP,
				VPNLastAssignedIP: serverConfig.ServerVPNStartIP,
			}
			fmt.Printf("Inserting SCIONLab AS configuration into database: %v\n", newSLS)
			if err := newSLS.Insert(); err != nil {
				return fmt.Errorf("ERROR: Cannot insert SCIONLab AS configuration into database: %v", err)
			}
		} else {
			return fmt.Errorf("ERROR: Cannot get SCIONLab AS configuration from database: %v", err)
		}
	} else { // Server exists and needs to be updated
		sls.IP = serverConfig.IP
		sls.VPNIP = serverConfig.VPNIP
		sls.UserPort = serverConfig.UserPort
		if sls.LastAssignedPort < serverConfig.ServerStartPort {
			sls.LastAssignedPort = serverConfig.ServerStartPort
		}
		if sls.VPNLastAssignedIP == "" || utility.IPToInt(sls.VPNLastAssignedIP) < utility.IPToInt(
			serverConfig.ServerVPNStartIP) {
			sls.VPNLastAssignedIP = serverConfig.ServerVPNStartIP
		}

		fmt.Printf("Updating SCIONLab AS configuration in database: %v\n", sls)
		if err := sls.Update(); err != nil {
			return fmt.Errorf("ERROR: Cannot update SCIONLab AS configuration in database: %v", err)
		}
	}

	return nil
}
