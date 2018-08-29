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
	"testing"
)

var (
	u1Email = "mail1"
	u2Email = "mail2"
	u3Email = "mail3"

	// SCIONLabASes
	as1 = &SCIONLabAS{
		UserEmail: u1Email,
		PublicIP:  "1.2.3.4",
		StartPort: 50000,
		ISD:       1,
		ASID:      1,
		Status:    Active,
		Type:      Box,
	}
	as2 = &SCIONLabAS{
		UserEmail: u2Email,
		StartPort: 50000,
		ISD:       1,
		ASID:      2,
		Status:    Inactive,
		Type:      VM,
	}
	as3 = &SCIONLabAS{
		UserEmail: u3Email,
		PublicIP:  "6.4.9.4",
		StartPort: 50000,
		ISD:       2,
		ASID:      5,
		Status:    Update,
		Type:      Dedicated,
		Core:      true,
	}

	// Attachment points
	ap1 = &AttachmentPoint{
		VPNIP:      "10.0.0.1",
		StartVPNIP: "10.0.0.2",
		EndVPNIP:   "10.0.0.19",
		AS:         as1,
	}
	ap2 = &AttachmentPoint{
		VPNIP:      "62.0.0.1",
		StartVPNIP: "62.0.0.2",
		EndVPNIP:   "62.0.255.254",
		AS:         as2,
	}

	// Connections
	cn1 = Connection{
		JoinIP:        "10.0.0.3",
		RespondIP:     ap1.VPNIP,
		JoinAS:        as2,
		RespondAP:     ap1,
		JoinBRID:      2,
		RespondBRID:   6,
		Linktype:      Parent,
		IsVPN:         true,
		JoinStatus:    Create,
		RespondStatus: Active,
	}
	cn2 = Connection{
		JoinIP:        as1.PublicIP,
		RespondIP:     as3.PublicIP,
		JoinAS:        as1,
		RespondAP:     ap2,
		JoinBRID:      2,
		RespondBRID:   1,
		Linktype:      Parent,
		IsVPN:         false,
		JoinStatus:    Remove,
		RespondStatus: Removed,
	}
	cn3 = Connection{
		JoinIP:        "62.0.0.53",
		RespondIP:     ap2.VPNIP,
		JoinAS:        as2,
		RespondAP:     ap2,
		JoinBRID:      4,
		RespondBRID:   7,
		Linktype:      Parent,
		IsVPN:         true,
		JoinStatus:    Active,
		RespondStatus: Create,
	}
)

func TestSCIONLabAS(t *testing.T) {
	// Insert users
	u1, err := RegisterUser("ac1", "ETH", u1Email, "pw1", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := RegisterUser("ac2", "ETH", u2Email, "pw2", "c", "d")
	if err != nil {
		t.Fatal(err)
	}
	u3, err := RegisterUser("ac3", "ETH", u3Email, "pw3", "f", "g")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Insert SCIONLabASes", func(t *testing.T) {
		err := as1.Insert()
		if err != nil {
			t.Fatal(err)
		}
		err = as2.Insert()
		if err != nil {
			t.Fatal(err)
		}
		err = as3.Insert()
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Insert APs", func(t *testing.T) {
		// AS1 & AS3 are attachment Points
		err := ap1.Insert()
		if err != nil {
			t.Fatal(err)
		}
		err = ap2.Insert()
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Insert Connections", func(t *testing.T) {
		// Insert Connections
		err := cn1.Insert()
		if err != nil {
			t.Fatal(err)
		}
		err = cn2.Insert()
		if err != nil {
			t.Fatal(err)
		}
		err = cn3.Insert()
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Find SCIONLabASes", func(t *testing.T) {
		// Test FindSCIONLabASByIA
		s1, err := FindSCIONLabASByIAString("1-1")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("FindSCIONLabASByIA 1-1: %v", s1)
		t.Logf("FindSCIONLabASByIA AP 1-1: %v", s1.AP)
		s3, err := FindSCIONLabASByIAString("2-5")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("FindSCIONLabASByIA 2-5: %v", s3)
		t.Logf("FindSCIONLabASByIA AP 2-5: %v", s3.AP)
		// Test FindSCIONLabASesByUserEmailAndType
		list, err := FindSCIONLabASesByUserEmailAndType("mail2", VM)
		if err != nil {
			t.Fatal(err)
		}
		for _, as := range list {
			t.Logf("FindSCIONLabASesByUserEmailAndType mail2, AS: %v", as)
		}
		// Test FindSCIONLabASesByUserEmail
		smail1, err := FindSCIONLabASesByUserEmail("mail1")
		if err != nil {
			t.Fatal(err)
		}
		for _, as := range smail1 {
			t.Logf("FindSCIONLabASesByUserEmail mail1: %v", as)
		}
		// Test FindSCIONLabASesByIP
		sIP, err := FindSCIONLabASesByIP("1.2.3.4")
		if err != nil {
			t.Fatal(err)
		}
		for _, sIP1 := range sIP {
			t.Logf("FindSCIONLabASesByIP 1.2.3.4: %v", sIP1)
		}
		// Test GetAllAPS
		APList, err := GetAllAPs()
		if err != nil {
			t.Fatal(err)
		}
		for _, ap := range APList {
			t.Logf("GetAllAPs: %v", ap)
		}
		// Test FindSCIONLabASByIAInt
		as, err := FindSCIONLabASByIAInt(1, 2)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("FindSCIONLabASByIAInt 1-2: %v", as)
	})

	t.Run("Find and update Connections", func(t *testing.T) {
		s1 := as1
		s2 := as2
		s3 := as3

		// Test GetConnectionInfo for all ASes
		cns1, err := s1.GetConnectionInfo()
		if err != nil {
			t.Fatal(err)
		}
		for _, cn := range cns1 {
			t.Logf("Connection s1: %v", cn)
		}
		cns2, err := s2.GetConnectionInfo()
		if err != nil {
			t.Fatal(err)
		}
		for _, cn := range cns2 {
			t.Logf("Connection s2: %v", cn)
		}
		cns3, err := s3.GetConnectionInfo()
		if err != nil {
			t.Fatal(err)
		}
		for _, cn := range cns3 {
			t.Logf("Connection s3: %v", cn)
		}

		// Test UpdateDBConnection
		s1.PublicIP = "CONNECTIONTEST"
		cns1[0].BRID = 60000
		cns1[0].Status = Update
		err = s1.Update()
		if err != nil {
			t.Fatal(err)
		}
		err = s1.UpdateDBConnectionFromJoinConnInfo(&cns1[0])
		if err != nil {
			t.Fatal(err)
		}
		s2.PublicIP = "CONNECTIONTEST"
		cns2[0].BRID = 60000
		cns2[0].Status = Update
		err = s2.Update()
		if err != nil {
			t.Fatal(err)
		}
		err = s2.UpdateDBConnectionFromJoinConnInfo(&cns2[0])
		if err != nil {
			t.Fatal(err)
		}
		cns1, err = s1.GetConnectionInfo()
		if err != nil {
			t.Fatal(err)
		}
		for _, cn := range cns1 {
			t.Logf("Connection s1: %v", cn)
		}
		cns2, err = s2.GetConnectionInfo()
		if err != nil {
			t.Fatal(err)
		}
		for _, cn := range cns2 {
			t.Logf("Connection s2: %v", cn)
		}
		cns3, err = s3.GetConnectionInfo()
		if err != nil {
			t.Fatal(err)
		}
		for _, cn := range cns3 {
			t.Logf("Connection s3: %v", cn)
		}
	})

	// Delete entries
	if err := u1.Delete(); err != nil {
		t.Error(err)
	}
	if err := u2.Delete(); err != nil {
		t.Error(err)
	}
	if err := u3.Delete(); err != nil {
		t.Error(err)
	}
	if err := cn1.Delete(); err != nil {
		t.Error(err)
	}
	if err := cn2.Delete(); err != nil {
		t.Error(err)
	}
	if err := cn3.Delete(); err != nil {
		t.Error(err)
	}
	if err := ap1.Delete(); err != nil {
		t.Error(err)
	}
	if err := ap2.Delete(); err != nil {
		t.Error(err)
	}
	if err := as1.Delete(); err != nil {
		t.Error(err)
	}
	if err := as2.Delete(); err != nil {
		t.Error(err)
	}
	if err := as3.Delete(); err != nil {
		t.Error(err)
	}

}
