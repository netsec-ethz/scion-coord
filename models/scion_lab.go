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
	"github.com/netsec-ethz/scion/go/lib/addr"
	"strconv"
)

type AttachmentPoint struct {
	Id          uint64 `orm:"column(id);auto;pk"`
	VPNIP       string
	StartVPNIP  string
	EndVPNIP    string
	AS          *SCIONLabAS   `orm:"reverse(one)"`
	Connections []*Connection `orm:"reverse(many);index"` // List of Connections
}

type SCIONLabAS struct {
	Id          uint64           `orm:"column(id);auto;pk"`
	UserEmail   string           // Email address of the Owning user
	PublicIP    string           // IP address of the SCIONLabAS
	BindIP      string           // Used for VPN connections specific to each type of AS
	IA          string           // IA in the form "ISD-AS"
	Core        bool             `orm:"default(false)"` // Is this SCIONLabAS a core AS
	Status      uint8            `orm:"default(0)"`     // Status of the AS (i.e Active, Create, Update, Remove)
	Type        uint8            // Type of the AS: (VM, DEDICATED, BOX, SERVER)
	AP          *AttachmentPoint `orm:"null;rel(one);on_delete(set_null)"`
	Connections []*Connection    `orm:"reverse(many)"` // List of Connections
}

type Connection struct {
	Id           uint64           `orm:"column(id);auto;pk"`
	InitAS       *SCIONLabAS      `orm:"rel(fk)"` // AS which initiated the connection
	AcceptAP     *AttachmentPoint `orm:"rel(fk)"` // AS which accepted the connection
	InitIP       string
	AcceptIP     string
	InitPort     int    // Port used by initiating AS
	AcceptPort   int    // Port used by accepting AS
	Linktype     string // PARENT -> Acceptor is Parent
	IsVPN        bool
	InitStatus   string // "new", "up", "delete", "deleted"
	AcceptStatus string
}

func (ap *AttachmentPoint) Insert() error {
	_, err := o.Insert(ap)
	return err
}

func (ap *AttachmentPoint) Update() error {
	_, err := o.Update(ap)
	return err
}

func (slas *SCIONLabAS) Insert() error {
	_, err := o.Insert(slas)
	return err
}

func (slas *SCIONLabAS) Update() error {
	_, err := o.Update(slas)
	return err
}

func (cn *Connection) Insert() error {
	_, err := o.Insert(cn)
	return err
}

func (cn *Connection) Update() error {
	_, err := o.Update(cn)
	return err
}

func (slas *SCIONLabAS) getConnections() ([]*Connection, error) {
	_, err := o.LoadRelated(slas, "Connections")
	if slas.AP != nil && err == nil {
		APCns, err := slas.AP.getConnections()
		return append(slas.Connections, APCns...), err
	}
	return slas.Connections, err
}

func (ap *AttachmentPoint) getConnections() ([]*Connection, error) {
	_, err := o.LoadRelated(ap, "Connections")
	return ap.Connections, err
}

func (cn *Connection) getInitAS() *SCIONLabAS {
	v := new(SCIONLabAS)
	o.QueryTable(v).Filter("Id", cn.InitAS.Id).RelatedSel().One(v)
	return v
}

func (cn *Connection) getAcceptAS() *SCIONLabAS {
	v := new(AttachmentPoint)
	o.QueryTable(v).Filter("Id", cn.AcceptAP.Id).RelatedSel().One(v)
	o.LoadRelated(v, "AS")
	return v.AS
}

type ConnectionInfo struct {
	NeighborISD int
	NeighborAS  int
	NeighborIP  string
	LocalIP     string
	RemotePort  int
	LocalPort   int
	Linktype    string //"PARENT","CHILD"
	IsVPN       bool
	Status      string
}

// Returns a list of connectionInfo
// Contains all info needed to populate the topology file
func (slas *SCIONLabAS) GetConnectionInfo() ([]ConnectionInfo, error) {
	cns, err := slas.getConnections()
	if err != nil {
		return nil, err
	}
	var cnInfos []ConnectionInfo
	var cnInfo ConnectionInfo
	for _, cn := range cns {
		// Check if As is initiator or acceptor
		acceptAS := cn.getAcceptAS()
		initAS := cn.getInitAS()
		if initAS.Id == slas.Id {
			neighborIA, err := addr.IAFromString(acceptAS.IA)
			if err != nil {
				return cnInfos, err
			}
			cnInfo = ConnectionInfo{
				NeighborISD: neighborIA.I,
				NeighborAS:  neighborIA.A,
				NeighborIP:  cn.AcceptIP,
				LocalIP:     cn.InitIP,
				RemotePort:  cn.AcceptPort,
				LocalPort:   cn.InitPort,
				Linktype:    cn.Linktype,
				IsVPN:       cn.IsVPN,
				Status:      cn.InitStatus,
			}
		} else {
			var linktype = cn.Linktype
			if cn.Linktype == "PARENT" {
				linktype = "CHILD"
			}
			neighborIA, err := addr.IAFromString(initAS.IA)
			if err != nil {
				return cnInfos, err
			}
			cnInfo = ConnectionInfo{
				NeighborISD: neighborIA.I,
				NeighborAS:  neighborIA.A,
				NeighborIP:  cn.InitIP,
				LocalIP:     cn.AcceptIP,
				RemotePort:  cn.InitPort,
				LocalPort:   cn.AcceptPort,
				Linktype:    linktype,
				IsVPN:       cn.IsVPN,
				Status:      cn.AcceptStatus,
			}
		}
		cnInfos = append(cnInfos, cnInfo)
	}
	return cnInfos, err
}

// Find SCIONLabAses by UserEmail
func FindSCIONLabASesByUserEmail(email string) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("UserEmail", email).RelatedSel().All(&v)
	return v, err
}

// Find SCIONLabAS by UserEmail and Type
// ASSUMPTION: only one type of AS for every user
// TODO (@philippmao, mlegner) Support multiple of same type ASes for a user
func FindSCIONLabASByUEmailAndType(email string, Type uint8) (*SCIONLabAS, error) {
	v := new(SCIONLabAS)
	err := o.QueryTable(v).Filter("UserEmail", email).Filter("Type", Type).RelatedSel().One(v)
	return v, err
}

// Find SCIONLabAS by the ia
func FindSCIONLabASByIA(ia *addr.ISD_AS) (*SCIONLabAS, error) {
	v := new(SCIONLabAS)
	err := o.QueryTable(v).Filter("IA__startswith", strconv.Itoa(ia.I)).Filter("IA__endswith", strconv.Itoa(ia.A)).RelatedSel().One(v)
	return v, err
}

// Find SCIONLabAS by the Public IP
func FindSCIONLabASesByIP(ip string) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("PublicIP", ip).RelatedSel().All(&v)
	return v, err
}
