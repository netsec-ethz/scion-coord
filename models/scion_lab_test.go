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
		BindIP:    "10.0.0.15",
		StartPort: 50000,
		Isd:       1,
		As:        1,
		Status:    ACTIVE,
		Type:      BOX,
	}
	slas2 := &SCIONLabAS{
		UserMail:  u2.Email,
		BindIP:    "127.0.0.1",
		StartPort: 50000,
		Isd:       1,
		As:        2,
		Status:    INACTIVE,
		Type:      VM,
	}
	slas3 := &SCIONLabAS{
		UserMail:  u3.Email,
		PublicIP:  "6.4.9.4",
		BindIP:    "127.0.0.1",
		StartPort: 50000,
		Isd:       2,
		As:        5,
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
		InitIP:       "10.0.0.3",
		AcceptIP:     slas1.AP.VPNIP,
		InitAS:       slas2,
		AcceptAP:     ap1,
		InitBrId:     2,
		AcceptBrId:   6,
		Linktype:     PARENT,
		IsVPN:        true,
		InitStatus:   CREATE,
		AcceptStatus: ACTIVE,
	}
	cn2 := Connection{
		InitIP:       slas1.PublicIP,
		AcceptIP:     slas3.PublicIP,
		InitAS:       slas1,
		AcceptAP:     ap2,
		InitBrId:     2,
		AcceptBrId:   1,
		Linktype:     PARENT,
		IsVPN:        false,
		InitStatus:   REMOVE,
		AcceptStatus: REMOVED,
	}
	cn3 := Connection{
		InitIP:       "62.0.0.53",
		AcceptIP:     slas3.AP.VPNIP,
		InitAS:       slas2,
		AcceptAP:     ap2,
		InitBrId:     4,
		AcceptBrId:   7,
		Linktype:     PARENT,
		IsVPN:        true,
		InitStatus:   ACTIVE,
		AcceptStatus: CREATE,
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
