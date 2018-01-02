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
	slas1 := &SCIONLabAS{
		UserMail:  u1.Email,
		PublicIP:  "1.2.3.4",
		StartPort: 50000,
		ISD:       1,
		AS:        1,
		Status:    ACTIVE,
		Type:      BOX,
	}
	slas2 := &SCIONLabAS{
		UserMail:  u2.Email,
		StartPort: 50000,
		ISD:       1,
		AS:        2,
		Status:    INACTIVE,
		Type:      VM,
	}
	slas3 := &SCIONLabAS{
		UserMail:  u3.Email,
		PublicIP:  "6.4.9.4",
		StartPort: 50000,
		ISD:       2,
		AS:        5,
		Status:    UPDATE,
		Type:      DEDICATED,
		Core:      true,
	}
	err = slas1.Insert()
	if err != nil {
		t.Fatal(err)
	}
	err = slas2.Insert()
	if err != nil {
		t.Fatal(err)
	}
	err = slas3.Insert()
	if err != nil {
		t.Fatal(err)
	}
	// SLAS1 & 3 are attachment Points
	ap1 := &AttachmentPoint{
		VPNIP:      "10.0.0.1",
		StartVPNIP: "10.0.0.2",
		EndVPNIP:   "10.0.0.19",
	}
	ap2 := &AttachmentPoint{
		VPNIP:      "62.0.0.1",
		StartVPNIP: "62.0.0.2",
		EndVPNIP:   "62.0.0.254",
	}
	err = ap1.Insert()
	if err != nil {
		t.Fatal(err)
	}
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
	cn1 := Connection{
		JoinIP:        "10.0.0.3",
		RespondIP:     slas1.AP.VPNIP,
		JoinAS:        slas2,
		RespondAP:     ap1,
		JoinBRID:      2,
		RespondBRID:   6,
		Linktype:      PARENT,
		IsVPN:         true,
		JoinStatus:    CREATE,
		RespondStatus: ACTIVE,
	}
	cn2 := Connection{
		JoinIP:        slas1.PublicIP,
		RespondIP:     slas3.PublicIP,
		JoinAS:        slas1,
		RespondAP:     ap2,
		JoinBRID:      2,
		RespondBRID:   1,
		Linktype:      PARENT,
		IsVPN:         false,
		JoinStatus:    REMOVE,
		RespondStatus: REMOVED,
	}
	cn3 := Connection{
		JoinIP:        "62.0.0.53",
		RespondIP:     slas3.AP.VPNIP,
		JoinAS:        slas2,
		RespondAP:     ap2,
		JoinBRID:      4,
		RespondBRID:   7,
		Linktype:      PARENT,
		IsVPN:         true,
		JoinStatus:    ACTIVE,
		RespondStatus: CREATE,
	}
	err = cn1.Insert()
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
	// Test FindSCIONLabASByTypeUserEmail
	list, err := FindSCIONLabASesByUEmailAndType("mail2", VM)
	if err != nil {
		t.Fatal(err)
	}
	var s2 SCIONLabAS
	for _, as := range list {
		t.Logf("FindSCIONLabASByUmailAndType mail2, VM: %v", as)
		s2 = as
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
	// Test GetConnectionInfo for all Ases
	cns1, err := s1.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns1 {
		t.Log("Connection s1: %v", cn)
	}
	cns2, err := s2.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns2 {
		t.Log("Connection s2: %v", cn)
	}
	cns3, err := s3.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns3 {
		t.Log("Connection s3: %v", cn)
	}
	// Test UpdateDBConnection
	s1.PublicIP = "CONNECTIONTEST"
	cns1[0].BRID = 99999
	cns1[0].Status = UPDATE
	err = s1.Update()
	if err != nil {
		t.Fatal(err)
	}
	err = s1.UpdateDBConnection(cns1[0])
	if err != nil {
		t.Fatal(err)
	}
	s2.PublicIP = "CONNECTIONTEST"
	cns2[0].BRID = 99999
	cns2[0].Status = UPDATE
	err = s2.Update()
	if err != nil {
		t.Fatal(err)
	}
	err = s2.UpdateDBConnection(cns2[0])
	if err != nil {
		t.Fatal(err)
	}
	cns1, err = s1.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns1 {
		t.Log("Connection s1: %v", cn)
	}
	cns2, err = s2.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns2 {
		t.Log("Connection s2: %v", cn)
	}
	cns3, err = s3.GetConnectionInfo()
	if err != nil {
		t.Fatal(err)
	}
	for _, cn := range cns3 {
		t.Log("Connection s3: %v", cn)
	}

	// clean up

	if err := u1.Account.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := u2.Account.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := u3.Account.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := u1.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := u2.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := u3.Delete(); err != nil {
		t.Fatal(err)
	}
}
