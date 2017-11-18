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
	ID          uint64 `orm:"column(id);auto;pk"`
	VPNIP       string
	StartVPNIP  string
	EndVPNIP    string
	AS          *SCIONLabAS   `orm:"reverse(one)"`
	Connections []*Connection `orm:"reverse(many);index"` // List of Connections
}

// TODO (@philippmao, mlegner) Add function to get BindIP from Type
type SCIONLabAS struct {
	ID          uint64 `orm:"column(id);auto;pk"`
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
	ID            uint64           `orm:"column(id);auto;pk"`
	JoinAS        *SCIONLabAS      `orm:"rel(fk)"` // AS which initiated the connection
	RespondAP     *AttachmentPoint `orm:"rel(fk)"` // AS which accepted the connection
	JoinIP        string
	RespondIP     string
	JoinBrId      int   // Id of the Initiator Border router, Port = StartPort + Id
	RespondBrId   int   // Id of the Acceptor Border router
	Linktype      uint8 // PARENT -> Acceptor is Parent
	IsVPN         bool
	JoinStatus    uint8
	RespondStatus uint8
	Created       time.Time
	Updated       time.Time
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
	var v []*Connection
	if err != nil {
		return v, err
	}
	v = append(v, slas.Connections...)
	if slas.AP != nil {
		APCns, err := slas.AP.getConnections()
		if err != nil {
			return v, err
		}
		v = append(v, APCns...)
	}
	return v, err
}

func (ap *AttachmentPoint) getConnections() ([]*Connection, error) {
	_, err := o.LoadRelated(ap, "Connections")
	return ap.Connections, err
}

func (cn *Connection) getJoinAS() *SCIONLabAS {
	v := new(SCIONLabAS)
	o.QueryTable(v).Filter("Id", cn.JoinAS.ID).RelatedSel().One(v)
	return v
}

func (cn *Connection) getRespondAS() *SCIONLabAS {
	v := new(AttachmentPoint)
	o.QueryTable(v).Filter("Id", cn.RespondAP.ID).RelatedSel().One(v)
	o.LoadRelated(v, "AS")
	return v.AS
}

type ConnectionInfo struct {
	CnID        uint64 // Used to find the BorderRouter
	NeighborISD int
	NeighborAS  int
	NeighborIP  string
	LocalIP     string
	BindIP      string
	BrID        int
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
		respondAS := cn.getRespondAS()
		joinAS := cn.getJoinAS()
		if joinAS.ID == slas.ID {
			if err != nil {
				return cnInfos, err
			}
			cnInfo = ConnectionInfo{
				CnID:        cn.ID,
				NeighborISD: respondAS.Isd,
				NeighborAS:  respondAS.As,
				NeighborIP:  cn.RespondIP,
				LocalIP:     cn.JoinIP,
				BindIP:      joinAS.BindIP,
				BrID:        cn.JoinBrId,
				RemotePort:  respondAS.StartPort + cn.RespondBrId,
				LocalPort:   joinAS.StartPort + cn.JoinBrId,
				Linktype:    cn.Linktype,
				IsVPN:       cn.IsVPN,
				Status:      cn.JoinStatus,
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
				NeighborISD: joinAS.Isd,
				NeighborAS:  joinAS.As,
				NeighborIP:  cn.JoinIP,
				LocalIP:     cn.RespondIP,
				BindIP:      respondAS.BindIP,
				BrID:        cn.RespondBrId,
				RemotePort:  joinAS.StartPort + cn.JoinBrId,
				LocalPort:   respondAS.StartPort + cn.RespondBrId,
				Linktype:    linktype,
				IsVPN:       cn.IsVPN,
				Status:      cn.RespondStatus,
			}
		}
		cnInfos = append(cnInfos, cnInfo)
	}
	return cnInfos, err
}

// Update Status of a Connection using a ConnectionInfo Object
func (slas *SCIONLabAS) UpdateDBConnection(cnInfo ConnectionInfo) error {
	cn := new(Connection)
	err := o.QueryTable(cn).Filter("ID", cnInfo.CnID).RelatedSel().One(cn)
	if err != nil {
		return err
	}
	respondAS := cn.getRespondAS()
	joinAS := cn.getJoinAS()
	if joinAS.ID == slas.ID {
		if !cn.IsVPN {
			cn.JoinIP = slas.PublicIP
		}
		cn.JoinStatus = cnInfo.Status
		// If the Connection is removed status REMOVE has to be set for both parties
		if cnInfo.Status == REMOVE {
			cn.RespondStatus = cnInfo.Status
		}
		cn.JoinBrId = cnInfo.BrID
	}
	if respondAS.ID == slas.ID {
		if !cn.IsVPN {
			cn.RespondIP = slas.PublicIP
		}
		cn.RespondStatus = cnInfo.Status
		if cnInfo.Status == REMOVE {
			cn.JoinStatus = cnInfo.Status
		}
		cn.RespondBrId = cnInfo.BrID
	}
	if err := cn.Update(); err != nil {
		return err
	}
	return nil
}

// Returns all Attachment Point ASes
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

// Find SCIONLabAS by the ISD AS int
func FindSCIONLabASByIAInt(Isd int, As int) (*SCIONLabAS, error) {
	v := new(SCIONLabAS)
	err := o.QueryTable(v).Filter("Isd", Isd).Filter("As", As).RelatedSel().One(v)
	return v, err
}

// Find SCIONLabASes by ISD
func FindSCIONLabAsesByIsd(isd int) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("Isd", isd).RelatedSel().All(&v)
	return v, err
}

// Find SCIONLabAS by the Public IP
func FindSCIONLabASesByIP(ip string) ([]SCIONLabAS, error) {
	var v []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("PublicIP", ip).RelatedSel().All(&v)
	return v, err
}
