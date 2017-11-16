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
	"testing"
)

const (
	INACTIVE = iota // 0
	ACTIVE
	CREATE
	UPDATE
	REMOVE
	BOX
	VM
	DEDICATED
	SERVER
)

/*type AttachmentPoint struct {
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
	Core        bool             // Is this SCIONLabAS a core AS
	Status      uint8            `orm:"default(0)"` // Status of the AS (i.e Active, Create, Update, Remove)
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
}*/

func Test(t *testing.T) {
	// Insert Users
	u1, err := RegisterUser("ac1", "ETH", "mail1", "pw", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := RegisterUser("ac2", "ETH", "mail2", "pw2", "c", "d")
	if err != nil {
		t.Fatal(err)
	}
	u3, err := RegisterUser("ac3", "ETH", "mail3", "pw3", "f", "g")
	if err != nil {
		t.Fatal(err)
	}
	slas1 := new(SCIONLabAS)
	slas1.IA = "1-1"
	slas1.PublicIP = "1.2.3.4"
	slas1.Status = ACTIVE
	slas1.Type = BOX
	slas1.UserEmail = u1.Email
	err = slas1.Insert()
	if err != nil {
		t.Fatal(err)
	}
	slas2 := new(SCIONLabAS)
	slas2.IA = "1-2"
	slas2.BindIP = "127.0.0.1"
	slas2.Status = INACTIVE
	slas2.Type = VM
	slas2.UserEmail = u2.Email
	err = slas2.Insert()
	if err != nil {
		t.Fatal(err)
	}
	slas3 := new(SCIONLabAS)
	slas3.IA = "2-5"
	slas3.PublicIP = "6.4.9.4"
	slas3.BindIP = "127.0.0.1"
	slas3.Status = UPDATE
	slas3.Type = DEDICATED
	slas3.Core = true
	slas3.UserEmail = u3.Email
	err = slas3.Insert()
	if err != nil {
		t.Fatal(err)
	}
	// SLAS1 & 3 are attachment Points
	ap1 := new(AttachmentPoint)
	ap1.VPNIP = "10.0.0.1"
	ap1.StartVPNIP = "10.0.0.2"
	ap1.EndVPNIP = "10.0.0.19"
	err = ap1.Insert()
	if err != nil {
		t.Fatal(err)
	}
	ap2 := new(AttachmentPoint)
	ap2.VPNIP = "62.0.0.1"
	ap2.StartVPNIP = "62.0.0.2"
	ap2.EndVPNIP = "62.0.0.254"
	err = ap2.Insert()
	if err != nil {
		t.Fatal(err)
	}
	// Link AP to SLAS
	slas1.AP = ap1
	err = slas1.Update()
	if err != nil {
		t.Fatal(err)
	}
	slas3.AP = ap2
	err = slas3.Update()
	if err != nil {
		t.Fatal(err)
	}
	// Insert Connections
	// 1-1 and 1-2 are connected via VPN
	cn1 := new(Connection)
	cn1.InitIP = "10.0.0.3"
	cn1.AcceptIP = slas1.AP.VPNIP
	cn1.InitAS = slas2
	cn1.AcceptAP = ap1
	cn1.AcceptPort = 50004
	cn1.InitPort = 50000
	cn1.IsVPN = true
	cn1.Linktype = "PARENT"
	cn1.AcceptStatus = "Up"
	cn1.InitStatus = "New"
	err = cn1.Insert()
	if err != nil {
		t.Fatal(err)
	}
	// 1-1 and 1-3 are connected
	cn2 := new(Connection)
	cn2.InitIP = slas1.PublicIP
	cn2.AcceptIP = slas3.PublicIP
	cn2.InitAS = slas1
	cn2.AcceptAP = ap2
	cn2.AcceptPort = 50002
	cn2.InitPort = 50001
	cn2.IsVPN = false
	cn2.Linktype = "PARENT"
	cn2.AcceptStatus = "Delete"
	cn2.InitStatus = "Delete"
	err = cn2.Insert()
	if err != nil {
		t.Fatal(err)
	}
	// 1-2 and 1-3 are connected via VPN
	cn3 := new(Connection)
	cn3.InitIP = "62.0.0.53"
	cn3.AcceptIP = slas3.AP.VPNIP
	cn3.InitAS = slas2
	cn3.AcceptAP = ap2
	cn3.AcceptPort = 50003
	cn3.InitPort = 50001
	cn3.IsVPN = true
	cn3.Linktype = "PARENT"
	cn3.AcceptStatus = "Up"
	cn3.InitStatus = "New"
	err = cn3.Insert()
	if err != nil {
		t.Fatal(err)
	}
	// Test FindSCIONLabASByIA
	ia, _ := addr.IAFromString("1-1")
	s1, err := FindSCIONLabASByIA(ia)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("FindSCIONLabASByIA 1-1: %v", s1)
	t.Logf("FindSCIONLabASByIA AP 1-1: %v", s1.AP)
	ia, _ = addr.IAFromString("2-5")
	s3, err := FindSCIONLabASByIA(ia)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("FindSCIONLabASByIA 2-5: %v", s3)
	t.Logf("FindSCIONLabASByIA AP 2-5: %v", s3.AP)
	// Test FindSCIONLabASByTypeUserEmail
	s2, err := FindSCIONLabASByUEmailAndType("mail2", VM)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("FindSCIONLabASByIA mail2, VM: %v", s2)
	s2.UserEmail = "mail1"
	err = s2.Update()
	if err != nil {
		t.Fatal(err)
	}
	// Test FindSCIONLabASesByUserEmail
	smail1, err := FindSCIONLabASesByUserEmail("mail1")
	if err != nil {
		t.Fatal(err)
	}
	for _, slas := range smail1 {
		t.Logf("FindSCIONLabAsesByUserEmail mail1: %v", slas)
	}
	// Test FindSCIONLabAsesByIP
	sIP, err := FindSCIONLabASesByIP("1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	for _, sIP1 := range sIP {
		t.Logf("FindSCIONLabAsesByIP 1.2.3.4: %v", sIP1)
	}
	// Test GetConnectionInfo for all Ases
	cns, err := s1.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns {
		t.Log("Connection s1: %v", cn)
	}
	cns, err = s2.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns {
		t.Log("Connection s2: %v", cn)
	}
	cns, err = s3.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns {
		t.Log("Connection s3: %v", cn)
	}

}
