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
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
)

// TODO(mlegner): Some of the functions here may not be optimally efficient

type AttachmentPoint struct {
	ID          uint64        `orm:"column(id);auto;pk"`
	HasVPN      bool          `orm:"column(has_vpn);default(1)"`
	VPNPort     uint16        `orm:"column(vpn_port);default(1194)"`
	VPNIP       string        `orm:"column(vpn_ip)"`
	StartVPNIP  string        `orm:"column(start_vpn_ip)"`
	EndVPNIP    string        `orm:"column(end_vpn_ip)"`
	AS          *SCIONLabAS   `orm:"column(as_id);rel(one);on_delete(cascade)"`
	Connections []*Connection `orm:"reverse(many);index"` // List of Connections
}

// TODO(philippmao, mlegner): Link SCIONLabAS to user model?
// TODO(mlegner): Maybe it would make more sense to replace the user by an account here
type SCIONLabAS struct {
	ID          uint64           `orm:"column(id);auto;pk"`
	UserEmail   string           // Owner of the AS
	PublicIP    string           `orm:"column(public_ip)"` // IP address of the AS; can be empty in case of VPN-based setups
	StartPort   uint16           // First port used for border routers
	ISD         addr.ISD         `orm:"column(isd);default(0)"` // 0 means no ISD is joined
	ASID        addr.AS          `orm:"column(as_id)"`
	Core        bool             `orm:"default(false)"` // Is this SCIONLabAS a core AS
	Label       string           // Optional label for this AS (can be chosen by the user)
	Status      uint8            `orm:"default(0)"` // Status of the AS: Active, Create, ...
	Type        uint8            `orm:"default(0)"` // Type of the AS: Box, VM, Dedicated, ...
	AP          *AttachmentPoint `orm:"null;reverse(one)"`
	Credits     int64            // Credits in virtual credit system
	Branch      string           `orm:"default(scionlab)"` // Update branch the AS is tracking ("scionlab", "scionlab_testing", "none")
	Created     time.Time        // When the AS was first created
	Updated     time.Time        // Last time the configuration was modified or the AS called `ConfirmUpdate`
	Connections []*Connection    `orm:"reverse(many)"` // List of Connections
	RemapStatus string           `orm:"size(1000);type(json);null"`
}

type Connection struct {
	ID            uint64           `orm:"column(id);auto;pk"`
	JoinAS        *SCIONLabAS      `orm:"column(join_as);rel(fk)"`    // AS which initiated the connection
	RespondAP     *AttachmentPoint `orm:"column(respond_ap);rel(fk)"` // AS which accepted the connection
	JoinIP        string           `orm:"column(join_ip)"`            // IP address used for the joining AS
	RespondIP     string           `orm:"column(respond_ip)"`         // IP address used for the responding AS
	JoinBRID      uint16           `orm:"column(join_br_id)"`         // ID of the joining border router, Port = StartPort + BRID
	RespondBRID   uint16           `orm:"column(respond_br_id)"`      // ID of the responding AS's border router
	Linktype      uint8            // role of the responding AS
	IsVPN         bool             `orm:"column(is_vpn)"`
	JoinStatus    uint8
	RespondStatus uint8
	Created       time.Time
	Updated       time.Time
}

// Contains all info needed to populate the topology file
type ConnectionInfo struct {
	ID                   uint64 // Used to find the BorderRouter
	NeighborISD          addr.ISD
	NeighborAS           addr.AS
	NeighborIP           string
	NeighborUser         string
	NeighborStatus       uint8
	LocalIP              string
	BindIP               string
	BRID                 uint16
	NeighborBRID         uint16
	NeighborPort         uint16 // port of the neighbor's border router
	LocalPort            uint16 // port of the local border router
	Linktype             uint8  //"PARENT","CHILD"
	IsVPN                bool
	Status               uint8
	KeepASStatusOnUpdate bool // true if this WAS a connection to an AP, but it needs to be deleted in the AP
	CreatedOn            time.Time
}

func (cn *ConnectionInfo) IsCurrentConnection() bool {
	return !cn.KeepASStatusOnUpdate
}

func filterConnectionsByBeingCurrentStatus(cns []ConnectionInfo, active bool) []ConnectionInfo {
	var res []ConnectionInfo
	for _, cn := range cns {
		if cn.IsCurrentConnection() == active {
			res = append(res, cn)
		}
	}
	return res
}

func OnlyCurrentConnections(cns []ConnectionInfo) []ConnectionInfo {
	return filterConnectionsByBeingCurrentStatus(cns, true)
}
func OnlyNotCurrentConnections(cns []ConnectionInfo) []ConnectionInfo {
	return filterConnectionsByBeingCurrentStatus(cns, false)
}

func (as *SCIONLabAS) IA() addr.IA {
	return addr.IA{I: as.ISD, A: as.ASID}
}

func (as *SCIONLabAS) IAString() string {
	if as.ISD < 1 {
		return fmt.Sprintf("%v", as.ASID)
	}
	return utility.IAStringStandard(as.ISD, as.ASID)
}

func (as *SCIONLabAS) String() string {
	res := as.IAString()
	if as.Label != "" {
		res = fmt.Sprintf("%v (%v)", res, as.Label)
	}
	return res
}

// This function determines the IP address that are used for different SCION servers (CS, BS, PS)
func (as *SCIONLabAS) ServerIP() string {
	switch as.Type {
	case Infrastructure, Dedicated:
		return as.PublicIP
	case VM:
		return config.VMLocalIP
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
	if err == orm.ErrNoRows {
		return []*Connection{}, nil
	}
	return ap.Connections, err
}

func (as *SCIONLabAS) GetPortNumberFromBRID(brID uint16) uint16 {
	return as.StartPort + brID - 1
}

func (as *SCIONLabAS) GetFreeBRID() (uint16, error) {
	cns, err := as.GetConnectionInfo()
	if err != nil {
		return 0, fmt.Errorf("Error finding connections of AS %v: %v", as.IAString(), err)
	}
	length := len(cns)
	brIDs := make([]int, length)
	for i, cn := range cns {
		brIDs[i] = int(cn.BRID)
	}
	minBRID := 1
	if as.Type == Infrastructure {
		minBRID += config.ReservedBRsInfrastructure
	}
	id, err := utility.GetAvailableID(brIDs, minBRID, config.MaxBRID)
	return uint16(id), err
}

// TODO(mlegner): Avoid signed/unsigned casting; could be problematic if huge IP ranges are used
func (as *SCIONLabAS) GetFreeVPNIP() (string, error) {
	cns, err := as.GetRespondConnections()
	if err != nil {
		return "", fmt.Errorf("Error finding connections of AP %v: %v", as.IAString(), err)
	}
	var vpnIPs []int
	for _, cn := range cns {
		if cn.IsVPN {
			vpnIPs = append(vpnIPs, int(utility.IPToInt(cn.JoinIP)))
		}
	}
	newIP, err := utility.GetAvailableID(vpnIPs, int(utility.IPToInt(as.AP.StartVPNIP)),
		int(utility.IPToInt(as.AP.EndVPNIP)))
	return utility.IntToIP(uint32(newIP)), err
}

// Only returns the connections of the AS in its function as the joining AS
func (as *SCIONLabAS) GetJoinConnections() ([]*Connection, error) {
	_, err := o.LoadRelated(as, "Connections")
	if err == orm.ErrNoRows {
		return []*Connection{}, nil
	}
	return as.Connections, err
}

// Only returns the connections of the AS in its function as an AP
func (as *SCIONLabAS) GetRespondConnections() ([]*Connection, error) {
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
	joinCns, err := as.GetJoinConnections()
	if err != nil {
		return nil, err
	}
	resCns, err := as.GetRespondConnections()
	if err != nil {
		return nil, err
	}
	return append(joinCns, resCns...), nil
}

func (cn *Connection) GetJoinAS() *SCIONLabAS {
	// as := new(SCIONLabAS)
	// o.QueryTable(as).Filter("ID", cn.JoinAS.ID).RelatedSel().One(as)
	// return as
	// TODO, Question: if we have cn.JoinAS as the JoinAS, with the correct type, why
	// do we need to query the DB again? Below is a simpler approach:
	o.LoadRelated(cn, "JoinAS")
	return cn.JoinAS
}

func (cn *Connection) GetRespondAS() *SCIONLabAS {
	// TODO: Question: same as the above method. Now we only ensure the AS is loaded:
	if cn.RespondAP.AS == nil {
		o.LoadRelated(cn.RespondAP, "AS")
	}
	return cn.RespondAP.AS
}

// Returns a list of ConnectionInfo where the AS is the joining AS
func (as *SCIONLabAS) GetJoinConnectionInfo() ([]ConnectionInfo, error) {
	cns, err := as.GetJoinConnections()
	if err != nil {
		return nil, err
	}
	var cnInfo ConnectionInfo
	var cnInfos []ConnectionInfo
	for _, cn := range cns {
		respondAS := cn.GetRespondAS()
		joinAS := cn.GetJoinAS()
		// If the connection has been removed continue
		if cn.JoinStatus == Removed {
			continue
		}
		cnInfo = ConnectionInfo{
			ID:                   cn.ID,
			NeighborISD:          addr.ISD(respondAS.ISD),
			NeighborAS:           addr.AS(respondAS.ASID),
			NeighborIP:           cn.RespondIP,
			NeighborUser:         respondAS.UserEmail,
			NeighborStatus:       respondAS.Status,
			LocalIP:              cn.JoinIP,
			BindIP:               cn.JoinBindIP(),
			BRID:                 cn.JoinBRID,
			NeighborBRID:         cn.RespondBRID,
			NeighborPort:         respondAS.GetPortNumberFromBRID(cn.RespondBRID),
			LocalPort:            joinAS.GetPortNumberFromBRID(cn.JoinBRID),
			Linktype:             cn.Linktype,
			IsVPN:                cn.IsVPN,
			Status:               cn.JoinStatus,
			KeepASStatusOnUpdate: cn.RespondStatus == Remove && cn.JoinStatus == Remove,
			CreatedOn:            cn.Created,
		}
		cnInfos = append(cnInfos, cnInfo)
	}
	return cnInfos, nil
}

// Returns a list of ConnectionInfo where the AS is the responding AS
func (as *SCIONLabAS) GetRespondConnectionInfo() ([]ConnectionInfo, error) {
	cns, err := as.GetRespondConnections()
	if err != nil {
		return nil, err
	}
	var cnInfo ConnectionInfo
	var cnInfos []ConnectionInfo
	for _, cn := range cns {
		respondAS := cn.GetRespondAS()
		joinAS := cn.GetJoinAS()
		if cn.RespondStatus == Removed {
			continue
		}
		linktype := cn.Linktype
		if cn.Linktype == Parent {
			linktype = Child
		}
		cnInfo = ConnectionInfo{
			ID:                   cn.ID,
			NeighborISD:          addr.ISD(joinAS.ISD),
			NeighborAS:           addr.AS(joinAS.ASID),
			NeighborIP:           cn.JoinIP,
			NeighborUser:         joinAS.UserEmail,
			NeighborStatus:       joinAS.Status,
			LocalIP:              cn.RespondIP,
			BindIP:               cn.RespondBindIP(),
			BRID:                 cn.RespondBRID,
			NeighborBRID:         cn.JoinBRID,
			NeighborPort:         joinAS.GetPortNumberFromBRID(cn.JoinBRID),
			LocalPort:            respondAS.GetPortNumberFromBRID(cn.RespondBRID),
			Linktype:             linktype,
			IsVPN:                cn.IsVPN,
			Status:               cn.RespondStatus,
			KeepASStatusOnUpdate: cn.RespondStatus == Remove && cn.JoinStatus == Remove,
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
func (as *SCIONLabAS) GetJoinConnectionInfoToAS(apIA string) ([]ConnectionInfo, error) {
	cns, err := as.GetJoinConnectionInfo()
	if err != nil {
		return nil, err
	}
	var res []ConnectionInfo
	for _, cn := range cns {
		if utility.IAStringStandard(cn.NeighborISD, cn.NeighborAS) == apIA {
			res = append(res, cn)
		}
	}
	return res, err
}

// GetRespondConnectionInfoToAS returns a list where the AS is the responding AS (the AP), and the
// other AS is the user AS attached to it.
func (as *SCIONLabAS) GetRespondConnectionInfoToAS(otherAS addr.AS) ([]ConnectionInfo, error) {
	cns, err := as.GetRespondConnectionInfo()
	if err != nil {
		return nil, err
	}
	var res []ConnectionInfo
	for _, cn := range cns {
		if cn.NeighborAS == otherAS {
			res = append(res, cn)
		}
	}
	return res, err
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
func (as *SCIONLabAS) UpdateDBConnectionFromJoinConnInfo(cnInfo *ConnectionInfo) error {
	cn := new(Connection)
	err := o.QueryTable(cn).Filter("ID", cnInfo.ID).RelatedSel().One(cn)
	if err != nil {
		return err
	}
	cn.IsVPN = cnInfo.IsVPN
	cn.JoinIP = cnInfo.LocalIP
	cn.RespondIP = cnInfo.NeighborIP

	respondAS := cn.GetRespondAS()
	joinAS := cn.GetJoinAS()
	if joinAS.ID == as.ID {
		cn.JoinStatus = cnInfo.Status
		cn.RespondStatus = cnInfo.NeighborStatus
		cn.JoinBRID = cnInfo.BRID
	}
	if respondAS.ID == as.ID {
		cn.RespondStatus = cnInfo.Status
		cn.JoinStatus = cnInfo.NeighborStatus
		cn.RespondBRID = cnInfo.BRID
	}
	return cn.Update()
}

func (as *SCIONLabAS) UpdateDBConnectionFromRespondConnInfo(cnInfo *ConnectionInfo) error {
	cn := new(Connection)
	if err := o.QueryTable(cn).Filter("ID", cnInfo.ID).RelatedSel().One(cn); err != nil {
		return nil
	}
	cn.IsVPN = cnInfo.IsVPN
	cn.JoinIP = cnInfo.NeighborIP
	cn.RespondIP = cnInfo.LocalIP

	respondAS := cn.GetRespondAS()
	joinAS := cn.GetJoinAS()
	if joinAS.ID == as.ID {
		cn.RespondStatus = cnInfo.Status
		cn.JoinStatus = cnInfo.NeighborStatus
		cn.RespondBRID = cnInfo.BRID
	}
	if respondAS.ID == as.ID {
		cn.JoinStatus = cnInfo.Status
		cn.RespondStatus = cnInfo.NeighborStatus
		cn.JoinBRID = cnInfo.BRID
	}
	return cn.Update()
}

// Update both the SCIONLabAS and Connection tables
func (as *SCIONLabAS) UpdateASAndConnectionFromJoinConnInfo(cnInfo *ConnectionInfo) error {
	if err := as.UpdateDBConnectionFromJoinConnInfo(cnInfo); err != nil {
		return err
	}
	return as.Update()
}
func (as *SCIONLabAS) UpdateASAndConnectionFromRespondConnInfo(cnInfo *ConnectionInfo) error {
	if err := as.UpdateDBConnectionFromRespondConnInfo(cnInfo); err != nil {
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
func FindAllAPsByISD(isd addr.ISD) ([]*SCIONLabAS, error) {
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

// Find SCIONLabASes by AccountID; returns a slice of IA strings
func FindSCIONLabASesByAccountID(accountID string) (asStrings []string, err error) {
	a, err := FindAccountByAccountID(accountID)
	if err != nil {
		return
	}
	o.LoadRelated(a, "Users")
	for _, u := range a.Users {
		var ases []SCIONLabAS
		ases, err = FindSCIONLabASesByUserEmail(u.Email)
		if err != nil {
			return
		}
		for _, as := range ases {
			asStrings = append(asStrings, as.IAString())
		}
	}
	return
}

// Find SCIONLabAS by the IA string
func FindSCIONLabASByIAString(ia string) (*SCIONLabAS, error) {
	IA, err := addr.IAFromString(ia)
	if err != nil {
		return nil, err
	}
	return FindSCIONLabASByASID(IA.A)
}

// Find SCIONLabAS by the ISD AS int
func FindSCIONLabASByIAInt(isd addr.ISD, asID addr.AS) (*SCIONLabAS, error) {
	return FindSCIONLabASByASID(asID)
}

func FindSCIONLabASByASID(asID addr.AS) (*SCIONLabAS, error) {
	as := new(SCIONLabAS)
	err := o.QueryTable(as).Filter("ASID", asID).RelatedSel().One(as)
	if err != nil {
		return nil, err
	}
	o.LoadRelated(as, "AP")
	return as, nil
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
	ISD     addr.ISD
	ASID    addr.AS
	Core    bool
	Account *Account
	Credits int64
	Created time.Time
}

func (asInfo *ASInfo) Insert() error {
	if asInfo.Account == nil || asInfo.Account.Users == nil || len(asInfo.Account.Users) == 0 {
		return errors.New("no user found")
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
		Status:    Inactive,
		Type:      Infrastructure,
	}
	return newAS.Insert()
}

func convertSCIONLabASToASInfo(as *SCIONLabAS) (*ASInfo, error) {
	account, err := FindAccountByUserEmail(as.UserEmail)
	if err != nil {
		return nil, err
	}
	asInfo := ASInfo{
		ISD:     addr.ISD(as.ISD),
		ASID:    addr.AS(as.ASID),
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

func FindCoreASInfosByISD(isd addr.ISD) ([]ASInfo, error) {
	var ases []SCIONLabAS
	_, err := o.QueryTable(new(SCIONLabAS)).Filter("ISD", isd).Filter("Core", true).All(&ases)
	if err != nil {
		return nil, err
	}
	return convertSCIONLabASesToASInfos(ases)
}

func FindASInfosByISD(isd addr.ISD) ([]ASInfo, error) {
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

// Delete a connection between specified ASes
func (as *SCIONLabAS) DeleteConnectionFromDB(cnInfo *ConnectionInfo) error {
	cn := new(Connection)
	err := o.QueryTable(new(Connection)).Filter("ID", cnInfo.ID).One(cn)
	if err != nil {
		return err
	}
	return cn.Delete()
}

func (as *SCIONLabAS) FlagAllConnectionsToApToBeDeleted(apIA string) error {
	cns, err := as.GetJoinConnectionInfo()
	if err != nil {
		return fmt.Errorf("Error looking up connections of SCIONLab AS for AS %v: %v", as.IAString(), err)
	}
	// all connections from an AS flagged as new connection and oldAP need to end up (localST, remoteST) = (REMOVE,REMOVE)
	for _, cn := range cns {
		if utility.IAString(cn.NeighborISD, cn.NeighborAS) != apIA {
			continue
		}
		cn.Status = Remove
		cn.NeighborStatus = Remove
		err = as.UpdateDBConnectionFromJoinConnInfo(&cn)
		if err != nil {
			return fmt.Errorf("error updating previous connection ID %v: %v", cn.ID, err)
		}
	}
	return nil
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

func DeleteConnectionFromDB(connectionId uint64) error {
	_, err := o.Delete(&Connection{ID: connectionId})
	return err
}

// AreIDsFromScionLab checks the ISD and AS numbers against the standard
// you can find in https://github.com/scionproto/scion/wiki/ISD-and-AS-numbering , and returns
// true if they are okay for SCIONLab; false otherwise.
func (as *SCIONLabAS) AreIDsFromScionLab() bool {
	if as.ISD <= 15 || as.ASID <= 4294967295 {
		return false
	}
	return true
}

// GetMappingStatus returns the mapping status map that was stored in the DB for this AS
func (as *SCIONLabAS) GetMappingStatus() (map[string]interface{}, error) {
	status := make(map[string]interface{})
	var err error
	if as.RemapStatus != "" {
		err = json.Unmarshal([]byte(as.RemapStatus), &status)
	}
	return status, err
}

// SetMappingStatusAndSave JSON serializes the dictionary, stores it in the AS and writes to DB
func (as *SCIONLabAS) SetMappingStatusAndSave(status map[string]interface{}) error {
	marshalled, err := json.Marshal(status)
	if err != nil {
		return err
	}
	as.RemapStatus = string(marshalled)
	err = as.Update()
	return err
}

// GetRemapChallenge returns the stored challenge or a new one otherwise.
func (as *SCIONLabAS) GetRemapChallenge() (string, error) {
	status, err := as.GetMappingStatus()
	if err != nil {
		return "", err
	}
	challengeAsAny, hasIt := status["challenge"]
	if !hasIt {
		randomBytes := make([]byte, 512)
		_, err = rand.Read(randomBytes)
		if err != nil {
			return "", errors.New("Could not create challenge")
		}
		challengeAsAny = base64.StdEncoding.EncodeToString(randomBytes)
		status["challenge"] = challengeAsAny
		err = as.SetMappingStatusAndSave(status)
		if err != nil {
			return "", err
		}
	}
	return challengeAsAny.(string), nil
}
