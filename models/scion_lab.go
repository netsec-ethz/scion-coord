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

	"github.com/netsec-ethz/scion/go/lib/addr"
)

type AttachmentPoint struct {
	Id          uint64 `orm:"column(id);auto;pk"`
	VPNIP       string
	StartVPNIP  string
	EndVPNIP    string
	AS          *SCIONLabAS   `orm:"reverse(one)"`
	Connections []*Connection `orm:"reverse(many);index"` // List of Connections
}

// TODO (@philippmao, mlegner) Add function to get BindIP from Type
type SCIONLabAS struct {
	Id          uint64 `orm:"column(id);auto;pk"`
	UserMail    string // User linked to the AS
	PublicIP    string // IP address of the SCIONLabAS
	BindIP      string // Used for VPN connections specific to each type of AS
	StartPort   int
	Isd         int
	As          int
	Core        bool             `orm:"default(false)"` // Is this SCIONLabAS a core AS
	Status      uint8            `orm:"default(0)"`     // Status of the AS (i.e Active, Create, Update, Remove)
	Type        uint8            `orm:"default(0)"`     // Type of the AS: (VM, DEDICATED, BOX, SERVER)
	AP          *AttachmentPoint `orm:"null;rel(one);on_delete(set_null)"`
	Created     time.Time
	Updated     time.Time
	Connections []*Connection `orm:"reverse(many)"` // List of Connections
}

type Connection struct {
	Id           uint64           `orm:"column(id);auto;pk"`
	InitAS       *SCIONLabAS      `orm:"rel(fk)"` // AS which initiated the connection
	AcceptAP     *AttachmentPoint `orm:"rel(fk)"` // AS which accepted the connection
	InitIP       string
	AcceptIP     string
	InitBrId     int   // Id of the Initiator Border router, Port = StartPort + Id
	AcceptBrId   int   // Id of the Acceptor Border router
	Linktype     uint8 // PARENT -> Acceptor is Parent
	IsVPN        bool
	InitStatus   uint8 // "new", "up", "delete", "deleted"
	AcceptStatus uint8
	Created      time.Time
	Updated      time.Time
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
	slas.Created = time.Now().UTC()
	slas.Updated = time.Now().UTC()
	_, err := o.Insert(slas)
	return err
}

func (slas *SCIONLabAS) Update() error {
	slas.Updated = time.Now().UTC()
	_, err := o.Update(slas)
	return err
}

func (cn *Connection) Insert() error {
	cn.Created = time.Now().UTC()
	cn.Updated = time.Now().UTC()
	_, err := o.Insert(cn)
	return err
}

func (cn *Connection) Update() error {
	cn.Updated = time.Now().UTC()
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
	BindIP      string
	BrId        int
	RemotePort  int
	LocalPort   int
	Linktype    uint8 //"PARENT","CHILD"
	IsVPN       bool
	Status      uint8
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
			if err != nil {
				return cnInfos, err
			}
			cnInfo = ConnectionInfo{
				NeighborISD: acceptAS.Isd,
				NeighborAS:  acceptAS.As,
				NeighborIP:  cn.AcceptIP,
				LocalIP:     cn.InitIP,
				BindIP:      initAS.BindIP,
				BrId:        cn.InitBrId,
				RemotePort:  acceptAS.StartPort + cn.AcceptBrId,
				LocalPort:   initAS.StartPort + cn.InitBrId,
				Linktype:    cn.Linktype,
				IsVPN:       cn.IsVPN,
				Status:      cn.InitStatus,
			}
		} else {
			var linktype = cn.Linktype
			if cn.Linktype == PARENT {
				linktype = CHILD
			}
			if err != nil {
				return cnInfos, err
			}
			cnInfo = ConnectionInfo{
				NeighborISD: initAS.Isd,
				NeighborAS:  initAS.As,
				NeighborIP:  cn.InitIP,
				LocalIP:     cn.AcceptIP,
				BindIP:      acceptAS.BindIP,
				BrId:        cn.AcceptBrId,
				RemotePort:  initAS.StartPort + cn.InitBrId,
				LocalPort:   acceptAS.StartPort + cn.AcceptBrId,
				Linktype:    linktype,
				IsVPN:       cn.IsVPN,
				Status:      cn.AcceptStatus,
			}
		}
		cnInfos = append(cnInfos, cnInfo)
	}
	return cnInfos, err
}

// This Function looks for all Attachment Point ASes
func GetAllAPs() ([]*SCIONLabAS, error) {
	var v []AttachmentPoint
	var w []*SCIONLabAS
	_, err := o.QueryTable(new(AttachmentPoint)).RelatedSel().All(&v)
	if err != nil {
		return w, err
	}
	for _, ap := range v {
		o.LoadRelated(&ap, "AS")
		w = append(w, ap.AS)
	}
	return w, err
}

// Find SCIONLabAses by UserEmail
func FindSCIONLabASesByUserEmail(email string) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("UserMail", email).RelatedSel().All(&v)
	return v, err
}

// Find SCIONLabASes by UserEmail and Type
func FindSCIONLabASesByUEmailAndType(email string, Type uint8) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("UserMail", email).Filter("Type", Type).RelatedSel().All(&v)
	return v, err
}

// Find SCIONLabAS by the IA string
func FindSCIONLabASByIAString(ia string) (*SCIONLabAS, error) {
	v := new(SCIONLabAS)
	IA, err1 := addr.IAFromString(ia)
	if err1 != nil {
		return nil, err1
	}
	err := o.QueryTable(v).Filter("Isd", IA.I).Filter("As", IA.A).RelatedSel().One(v)
	return v, err
}

// Find SCIONLabAS by the Public IP
func FindSCIONLabASesByIP(ip string) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("PublicIP", ip).RelatedSel().All(&v)
	return v, err
}
