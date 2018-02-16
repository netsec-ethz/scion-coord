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

	"fmt"

	"errors"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
)

// TODO(mlegner): Some of the functions here may not be optimally efficient

type AttachmentPoint struct {
	ID          uint64 `orm:"column(id);auto;pk"`
	VPNIP       string
	StartVPNIP  string
	EndVPNIP    string
	AS          *SCIONLabAS   `orm:"column(as_id);rel(one);on_delete(cascade)"`
	Connections []*Connection `orm:"reverse(many);index"` // List of Connections
}

// TODO(philippmao, mlegner): Link SCIONLabAS to user model?
// TODO(mlegner): Maybe it would make more sense to replace the user by an account here
type SCIONLabAS struct {
	ID          uint64 `orm:"column(id);auto;pk"`
	UserEmail   string // Owner of the AS
	PublicIP    string // IP address of the SCIONLabAS; can be empty in case of VPN-based setups
	StartPort   uint16 // First port used for border routers
	ISD         int    `orm:"default(0)"` // 0 means no ISD is joined
	ASID        int
	Core        bool             `orm:"default(false)"` // Is this SCIONLabAS a core AS
	Label       string           // Optional label for this AS (can be chosen by the user)
	Status      uint8            `orm:"default(0)"` // Status of the AS: ACTIVE, CREATE, ...
	Type        uint8            `orm:"default(0)"` // Type of the AS: BOX, VM, DEDICATED, ...
	AP          *AttachmentPoint `orm:"null;reverse(one)"`
	Credits     int64
	Created     time.Time
	Updated     time.Time
	Connections []*Connection `orm:"reverse(many)"` // List of Connections
}

type Connection struct {
	ID            uint64           `orm:"column(id);auto;pk"`
	JoinAS        *SCIONLabAS      `orm:"rel(fk)"` // AS which initiated the connection
	RespondAP     *AttachmentPoint `orm:"rel(fk)"` // AS which accepted the connection
	JoinIP        string           // IP address used for the joining AS
	RespondIP     string           // IP address used for the responding AS
	JoinBRID      uint16           // ID of the joining border router, Port = StartPort + BRID
	RespondBRID   uint16           // ID of the responding AS's border router
	Linktype      uint8            // role of the responding AS
	IsVPN         bool
	JoinStatus    uint8
	RespondStatus uint8
	Created       time.Time
	Updated       time.Time
}

// Contains all info needed to populate the topology file
type ConnectionInfo struct {
	ID             uint64 // Used to find the BorderRouter
	NeighborISD    int
	NeighborAS     int
	NeighborIP     string
	NeighborUser   string
	NeighborStatus uint8
	LocalIP        string
	BindIP         string
	BRID           uint16
	NeighborPort   uint16 // port of the neighbor's border router
	LocalPort      uint16 // port of the local border router
	Linktype       uint8  //"PARENT","CHILD"
	IsVPN          bool
	Status         uint8
}

func (as *SCIONLabAS) IA() string {
	if as.ISD < 1 {
		return fmt.Sprintf("%v", as.ASID)
	}
	return utility.IAString(as.ISD, as.ASID)
}

func (as *SCIONLabAS) String() string {
	res := as.IA()
	if as.Label != "" {
		res = fmt.Sprintf("%v (%v)", as.IA(), as.Label)
	}
	return res
}

// This function determines the IP address that are used for different SCION servers (CS, BS, PS)
func (as *SCIONLabAS) ServerIP() string {
	switch as.Type {
	case INFRASTRUCTURE, DEDICATED:
		return as.PublicIP
	case VM:
		return config.VM_LOCAL_IP
	default:
		return "127.0.0.1"
	}
}

// This function determines the BindIP address used for the border router of a given connection
// TODO(mlegner): This should be replaced by an iptables rule and simply the ServerIP here
func (as *SCIONLabAS) BindIP(isVPN bool, connectionIP string) string {
	if isVPN {
		return connectionIP
	}
	return as.ServerIP()
}

func (cn *Connection) JoinBindIP() string {
	as := cn.JoinAS
	return as.BindIP(cn.IsVPN, cn.JoinIP)
}

func (cn *Connection) RespondBindIP() string {
	ap := cn.RespondAP
	o.LoadRelated(ap, "AS")
	if ap.AS == nil {
		return ""
	}
	return ap.AS.BindIP(cn.IsVPN, cn.RespondIP)
}

func (ap *AttachmentPoint) Insert() error {
	_, err := o.Insert(ap)
	return err
}

func (ap *AttachmentPoint) Update() error {
	_, err := o.Update(ap)
	return err
}

func (as *SCIONLabAS) Insert() error {
	as.Created = time.Now().UTC()
	as.Updated = time.Now().UTC()
	_, err := o.Insert(as)
	return err
}

func (as *SCIONLabAS) Update() error {
	as.Updated = time.Now().UTC()
	_, err := o.Update(as)
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

func (ap *AttachmentPoint) getConnections() ([]*Connection, error) {
	_, err := o.LoadRelated(ap, "Connections")
	return ap.Connections, err
}

func (as *SCIONLabAS) GetPortNumberFromBRID(brID uint16) uint16 {
	return as.StartPort + brID - 1
}

func (as *SCIONLabAS) GetFreeBRID() (uint16, error) {
	cns, err := as.GetConnectionInfo()
	if err != nil {
		return 0, fmt.Errorf("Error finding connections of AS %v: %v", as.IA(), err)
	}
	length := len(cns)
	brIDs := make([]int, length)
	for i, cn := range cns {
		brIDs[i] = int(cn.BRID)
	}
	minBRID := 1
	if as.Type == INFRASTRUCTURE {
		minBRID += config.RESERVED_BRS_INFRASTRUCTURE
	}
	id, err := utility.GetFreeID(brIDs, minBRID, config.MAX_BR_ID)
	return uint16(id), err
}

// TODO(mlegner): Avoid signed/unsigned casting; could be problematic if huge IP ranges are used
func (as *SCIONLabAS) GetFreeVPNIP() (string, error) {
	cns, err := as.getRespondConnections()
	if err != nil {
		return "", fmt.Errorf("Error finding connections of AP %v: %v", as.IA(), err)
	}
	vpnIPs := []int{}
	for _, cn := range cns {
		if cn.IsVPN {
			vpnIPs = append(vpnIPs, int(utility.IPToInt(cn.JoinIP)))
		}
	}
	newIP, err := utility.GetFreeID(vpnIPs, int(utility.IPToInt(as.AP.StartVPNIP)),
		int(utility.IPToInt(as.AP.EndVPNIP)))
	return utility.IntToIP(uint32(newIP)), err
}

// Only returns the connections of the AS in its function as the joining AS
func (as *SCIONLabAS) getJoinConnections() ([]*Connection, error) {
	if _, err := o.LoadRelated(as, "Connections"); err != nil {
		return nil, err
	}
	return as.Connections, nil
}

// Only returns the connections of the AS in its function as an AP
func (as *SCIONLabAS) getRespondConnections() ([]*Connection, error) {
	var cns []*Connection
	if as.AP != nil {
		APCns, err := as.AP.getConnections()
		if err != nil {
			return cns, err
		}
		cns = APCns
	}
	return cns, nil
}

// Returns all connections of the AS
func (as *SCIONLabAS) getAllConnections() ([]*Connection, error) {
	joinCns, err := as.getJoinConnections()
	if err != nil {
		return nil, err
	}
	resCns, err := as.getRespondConnections()
	if err != nil {
		return nil, err
	}
	return append(joinCns, resCns...), nil
}

func (cn *Connection) getJoinAS() *SCIONLabAS {
	as := new(SCIONLabAS)
	o.QueryTable(as).Filter("ID", cn.JoinAS.ID).RelatedSel().One(as)
	return as
}

func (cn *Connection) getRespondAS() *SCIONLabAS {
	ap := new(AttachmentPoint)
	o.QueryTable(ap).Filter("ID", cn.RespondAP.ID).RelatedSel().One(ap)
	o.LoadRelated(ap, "AS")
	return ap.AS
}

// Returns a list of ConnectionInfo where the AS is the joining AS
func (as *SCIONLabAS) GetJoinConnectionInfo() ([]ConnectionInfo, error) {
	cns, err := as.getJoinConnections()
	if err != nil {
		return nil, err
	}
	var cnInfo ConnectionInfo
	var cnInfos []ConnectionInfo
	for _, cn := range cns {
		respondAS := cn.getRespondAS()
		joinAS := cn.getJoinAS()
		// If the connection has been removed continue
		if cn.JoinStatus == REMOVED {
			continue
		}
		cnInfo = ConnectionInfo{
			ID:             cn.ID,
			NeighborISD:    respondAS.ISD,
			NeighborAS:     respondAS.ASID,
			NeighborIP:     cn.RespondIP,
			NeighborUser:   respondAS.UserEmail,
			NeighborStatus: respondAS.Status,
			LocalIP:        cn.JoinIP,
			BindIP:         cn.JoinBindIP(),
			BRID:           cn.JoinBRID,
			NeighborPort:   respondAS.GetPortNumberFromBRID(cn.RespondBRID),
			LocalPort:      joinAS.GetPortNumberFromBRID(cn.JoinBRID),
			Linktype:       cn.Linktype,
			IsVPN:          cn.IsVPN,
			Status:         cn.JoinStatus,
		}
		cnInfos = append(cnInfos, cnInfo)
	}
	return cnInfos, nil
}

// Returns a list of ConnectionInfo where the AS is the responding AS
func (as *SCIONLabAS) GetRespondConnectionInfo() ([]ConnectionInfo, error) {
	cns, err := as.getRespondConnections()
	if err != nil {
		return nil, err
	}
	var cnInfo ConnectionInfo
	var cnInfos []ConnectionInfo
	for _, cn := range cns {
		respondAS := cn.getRespondAS()
		joinAS := cn.getJoinAS()
		if cn.RespondStatus == REMOVED {
			continue
		}
		linktype := cn.Linktype
		if cn.Linktype == PARENT {
			linktype = CHILD
		}
		cnInfo = ConnectionInfo{
			ID:             cn.ID,
			NeighborISD:    joinAS.ISD,
			NeighborAS:     joinAS.ASID,
			NeighborIP:     cn.JoinIP,
			NeighborUser:   joinAS.UserEmail,
			NeighborStatus: joinAS.Status,
			LocalIP:        cn.RespondIP,
			BindIP:         cn.RespondBindIP(),
			BRID:           cn.RespondBRID,
			NeighborPort:   joinAS.GetPortNumberFromBRID(cn.JoinBRID),
			LocalPort:      respondAS.GetPortNumberFromBRID(cn.RespondBRID),
			Linktype:       linktype,
			IsVPN:          cn.IsVPN,
			Status:         cn.RespondStatus,
		}
		cnInfos = append(cnInfos, cnInfo)
	}
	return cnInfos, nil
}

// Returns a list of ConnectionInfo for all connections of the AS
func (as *SCIONLabAS) GetConnectionInfo() ([]ConnectionInfo, error) {
	joinCns, err := as.GetJoinConnectionInfo()
	if err != nil {
		return nil, err
	}
	resCns, err := as.GetRespondConnectionInfo()
	if err != nil {
		return nil, err
	}
	return append(joinCns, resCns...), nil
}

// Returns the connection of an AP to the specified AS
// TODO(mlegner): This function assumes that there can only be one connection between an AS/AP pair
func (as *SCIONLabAS) GetRespondConnectionInfoToAS(asIA string) (cn ConnectionInfo, err error) {
	cns, err := as.GetRespondConnectionInfo()
	if err != nil {
		return
	}
	for _, cn = range cns {
		if utility.IAString(cn.NeighborISD, cn.NeighborAS) == asIA {
			return
		}
	}
	err = fmt.Errorf("No connection found for the AS/AP pair %v/%v", asIA, as.IA())
	return
}

// Returns the connection of an AP to the specified AS
// TODO(mlegner): This function assumes that there can only be one connection between an AS/AP pair
func (as *SCIONLabAS) GetJoinConnectionInfoToAS(apIA string) (cn ConnectionInfo, err error) {
	cns, err := as.GetJoinConnectionInfo()
	if err != nil {
		return
	}
	for _, cn = range cns {
		if utility.IAString(cn.NeighborISD, cn.NeighborAS) == apIA {
			return
		}
	}
	err = orm.ErrNoRows
	return
}

// Takes the IA string as an input and returns all ConnectionInfos where the AS is the AP
func FindRespondConnectionInfoByIA(ia string) ([]ConnectionInfo, error) {
	as, err := FindSCIONLabASByIAString(ia)
	if err != nil {
		return nil, err
	}
	return as.GetRespondConnectionInfo()
}

// Update the Status of a Connection using a ConnectionInfo Object
func (as *SCIONLabAS) UpdateDBConnection(cnInfo *ConnectionInfo) error {
	cn := new(Connection)
	err := o.QueryTable(cn).Filter("ID", cnInfo.ID).RelatedSel().One(cn)
	if err != nil {
		return err
	}
	respondAS := cn.getRespondAS()
	joinAS := cn.getJoinAS()
	if joinAS.ID == as.ID {
		if !cn.IsVPN {
			cn.JoinIP = as.PublicIP
		}
		cn.JoinStatus = cnInfo.Status
		cn.RespondStatus = cnInfo.NeighborStatus
		cn.JoinBRID = cnInfo.BRID
	}
	if respondAS.ID == as.ID {
		if !cn.IsVPN {
			cn.RespondIP = as.PublicIP
		}
		cn.RespondStatus = cnInfo.Status
		cn.JoinStatus = cnInfo.NeighborStatus
		cn.RespondBRID = cnInfo.BRID
	}
	if err := cn.Update(); err != nil {
		return err
	}
	return nil
}

// Update both the SCIONLabAS and Connection tables
func (as *SCIONLabAS) UpdateASAndConnection(cnInfo *ConnectionInfo) error {
	if err := as.UpdateDBConnection(cnInfo); err != nil {
		return err
	}
	return as.Update()
}

// Returns all Attachment Point ASes
func GetAllAPs() ([]*SCIONLabAS, error) {
	var aps []*AttachmentPoint
	var ases []*SCIONLabAS
	_, err := o.QueryTable(new(AttachmentPoint)).RelatedSel().All(&aps)
	for _, ap := range aps {
		ases = append(ases, ap.AS)
	}
	return ases, err
}

// Returns all Attachment Point ASes in the given ISD
func FindAllAPsByISD(isd int) ([]*SCIONLabAS, error) {
	var aps []*AttachmentPoint
	var ases []*SCIONLabAS
	_, err := o.QueryTable(new(AttachmentPoint)).RelatedSel().All(&aps)
	for _, ap := range aps {
		if ap.AS.ISD == isd {
			ases = append(ases, ap.AS)
		}
	}
	return ases, err
}

// Find SCIONLabASes by UserEmail
func FindSCIONLabASesByUserEmail(email string) ([]SCIONLabAS, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("UserEmail", email).RelatedSel().All(&ases)
	return ases, err
}

// Find a single SCIONLabAS by UserEmail and the AS ID (can be specified as a string or int)
func FindSCIONLabASByUserEmailAndASID(email string, asID interface{}) (*SCIONLabAS, error) {
	as := new(SCIONLabAS)
	err := o.QueryTable(as).Filter("ASID", asID).Filter("UserEmail", email).RelatedSel().One(as)
	return as, err
}

// Find SCIONLabASes by UserEmail and Type
func FindSCIONLabASesByUserEmailAndType(email string, Type uint8) ([]SCIONLabAS, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("UserEmail", email).Filter("Type",
		Type).RelatedSel().All(&ases)
	return ases, err
}

// Find SCIONLabAS by the IA string
func FindSCIONLabASByIAString(ia string) (*SCIONLabAS, error) {
	as := new(SCIONLabAS)
	IA, err1 := addr.IAFromString(ia)
	if err1 != nil {
		return nil, err1
	}
	err := o.QueryTable(as).Filter("ISD", IA.I).Filter("ASID", IA.A).RelatedSel().One(as)
	o.LoadRelated(as, "AP")
	return as, err
}

// Find SCIONLabAS by the ISD AS int
func FindSCIONLabASByIAInt(isd int, asID int) (*SCIONLabAS, error) {
	as := new(SCIONLabAS)
	err := o.QueryTable(as).Filter("ISD", isd).Filter("ASID", asID).RelatedSel().One(as)
	return as, err
}

// Find SCIONLabAS by the Public IP
// TODO(mlegner): The PublicIP field can be empty; we need to be careful with this function
func FindSCIONLabASesByIP(ip string) ([]SCIONLabAS, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("PublicIP", ip).RelatedSel().All(&ases)
	return ases, err
}

// The following struct and functions are used in the mediation API functions
type ASInfo struct {
	ISD     int
	ASID    int
	Core    bool
	Account *Account
	Credits int64
	Created time.Time
}

func (asInfo *ASInfo) Insert() error {
	if asInfo.Account == nil || asInfo.Account.Users == nil || len(asInfo.Account.Users) == 0 {
		return errors.New("No user found")
	}
	user := asInfo.Account.Users[0] // using first user associated with account
	newAS := SCIONLabAS{
		UserEmail: user.Email,
		ISD:       asInfo.ISD,
		ASID:      asInfo.ASID,
		Core:      asInfo.Core,
		Credits:   asInfo.Credits,
		PublicIP:  "",
		StartPort: 50000,
		Status:    INACTIVE,
		Type:      INFRASTRUCTURE,
	}
	return newAS.Insert()
}

func convertSCIONLabASToASInfo(as *SCIONLabAS) (*ASInfo, error) {
	account, err := FindAccountByUserEmail(as.UserEmail)
	if err != nil {
		return nil, err
	}
	asInfo := ASInfo{
		ISD:     as.ISD,
		ASID:    as.ASID,
		Core:    as.Core,
		Account: account,
		Credits: as.Credits,
		Created: as.Created,
	}
	return &asInfo, nil
}

func convertSCIONLabASesToASInfos(ases []SCIONLabAS) (asInfos []ASInfo, err error) {
	var asInfo *ASInfo
	for _, as := range ases {
		asInfo, err = convertSCIONLabASToASInfo(&as)
		if err != nil {
			return
		}
		asInfos = append(asInfos, *asInfo)
	}
	return
}

func FindCoreASInfosByISD(isd int) ([]ASInfo, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("ISD", isd).Filter("Core", true).All(&ases)
	if err != nil {
		return nil, err
	}
	return convertSCIONLabASesToASInfos(ases)
}

func FindASInfosByISD(isd int) ([]ASInfo, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("ISD", isd).All(&ases)
	if err != nil {
		return nil, err
	}
	return convertSCIONLabASesToASInfos(ases)
}

func FindASInfoByIA(isdas string) (*ASInfo, error) {
	ia, err := addr.IAFromString(isdas)
	if err != nil {
		return nil, err
	}
	as := new(SCIONLabAS)
	err = o.QueryTable(as).Filter("ISD", ia.I).Filter("ASID", ia.A).RelatedSel().One(as)
	if err != nil {
		return nil, err
	}
	return convertSCIONLabASToASInfo(as)
}

func FindAllASInfos() ([]ASInfo, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).All(&ases)
	if err != nil {
		return nil, err
	}
	return convertSCIONLabASesToASInfos(ases)
}

func FindSCIONLabASByASInfo(asInfo ASInfo) (*SCIONLabAS, error) {
	return FindSCIONLabASByIAInt(asInfo.ISD, asInfo.ASID)
}

func (asInfo *ASInfo) String() string {
	return utility.IAString(asInfo.ISD, asInfo.ASID)
}

func (as *SCIONLabAS) Delete() error {
	_, err := o.Delete(as)
	return err
}

func (ap *AttachmentPoint) Delete() error {
	_, err := o.Delete(ap)
	return err
}

func (cn *Connection) Delete() error {
	_, err := o.Delete(cn)
	return err
}
