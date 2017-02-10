// Copyright 2016 ETH Zurich
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
	//	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func createTestUser(t *testing.T) *user {
	email := "testemail@scion.com"
	password := "pass"
	u, err := RegisterUser("scion-test-db", "Scion Test DB", email, password, "John", "Doe")
	if err != nil {
		t.Error(err)
	}
	return u
}

func TestJoinRequest(t *testing.T) {
	u := createTestUser(t)
	jReqIn := &JoinRequest{RequestId: 777, RespondIA: "100-100", SigPubKey: "sigkey",
		RequesterId: u.Account.AccountId, EncPubKey: "enckey", Status: PENDING}
	if err := jReqIn.Insert(); err != nil {
		t.Error("Failed to insert the test join request.", err)
	}
	jReqOut, err := FindJoinRequest(u.Account.AccountId, jReqIn.RequestId)
	if err != nil {
		t.Error("Failed to retrieve the test join request.", err)
	}
	assert.Equal(t, jReqIn.RequestId, jReqOut.RequestId)
	assert.Equal(t, jReqIn.RespondIA, jReqOut.RespondIA)
	assert.Equal(t, jReqIn.RequesterId, jReqOut.RequesterId)
	assert.Equal(t, jReqIn.SigPubKey, jReqOut.SigPubKey)
	assert.Equal(t, jReqIn.EncPubKey, jReqOut.EncPubKey)
	assert.Equal(t, jReqIn.Status, jReqOut.Status)
	if err := DeleteJoinRequest(u.Account.AccountId, jReqIn.RequestId); err != nil {
		t.Error("Failed to delete the test join request.", err)
	}
	if err := u.Delete(); err != nil {
		t.Error(err)
	}
}

func TestJoinReply(t *testing.T) {
	u := createTestUser(t)
	jRepIn := &JoinReply{RequestId: 1234, JoiningIA: "123-123", RespondIA: "321-312",
		RequesterId: u.Account.AccountId, JoiningIACertificate: "cert1",
		RespondIACertificate: "cert2", TRC: "trc"}
	if err := jRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test join reply.", err)
	}
	jRepOut, err := FindJoinReply(u.Account.AccountId, jRepIn.RequestId)
	if err != nil {
		t.Error("Failed to retrieve the test join reply.", err)
	}
	assert.Equal(t, jRepIn.Id, jRepOut.Id)
	assert.Equal(t, jRepIn.JoiningIA, jRepOut.JoiningIA)
	assert.Equal(t, jRepIn.RespondIA, jRepOut.RespondIA)
	assert.Equal(t, jRepIn.RequesterId, jRepOut.RequesterId)
	assert.Equal(t, jRepIn.JoiningIACertificate, jRepOut.JoiningIACertificate)
	assert.Equal(t, jRepIn.RespondIACertificate, jRepOut.RespondIACertificate)
	assert.Equal(t, jRepIn.TRC, jRepOut.TRC)
	if err := DeleteJoinReply(u.Account.AccountId, jRepIn.RequestId); err != nil {
		t.Error("Failed to delete the test join reply.", err)
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestConnRequest(t *testing.T) {
	u := createTestUser(t)
	cReqIn := &ConnRequest{RequestId: 999, RespondIA: "111-111", RequestIA: "222-222",
		AccountId: u.Account, RequesterCertificate: "test_cert", Info: "test_info",
		IP: "123.123.123.123", Port: 555, OverlayType: "UDP/IP", MTU: 1472, Bandwidth: 1000,
		Signature: "test_sig", Status: PENDING}
	if err := cReqIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection request.", err)
	}
	cReqOut, err := FindConnRequest(u.Account, cReqIn.RequestId)
	if err != nil {
		t.Error("Failed to retrieve the test connection request.", err)
	}
	assert.Equal(t, cReqIn.RequestId, cReqOut.RequestId)
	assert.Equal(t, cReqIn.IP, cReqOut.IP)
	assert.Equal(t, cReqIn.AccountId.Name, cReqOut.AccountId.Name)
	assert.Equal(t, cReqIn.OverlayType, cReqOut.OverlayType)
	if err := DeleteConnRequest(u.Account, cReqOut.RequestId); err != nil {
		t.Error("Failed to delete the test connection request.", err)
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestConnReply(t *testing.T) {
	u := createTestUser(t)
	cRepIn := &ConnReply{Id: 6, RequestId: 4321, RespondIA: "321-321", RequestIA: "678-678",
		AccountId: u.Account, Certificate: "test_cert", IP: "123.123.123.123", Port: 333,
		OverlayType: "UDP/IP", MTU: 1472, Bandwidth: 1000}
	if err := cRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection reply.", err)
	}
	cReps, err := FindConnRepliesByRequestIA(cRepIn.RequestIA)
	if err != nil {
		t.Error("Failed to retrieve the test connection reply.", err)
	}
	assert.Equal(t, 1, len(cReps))
	cRepOut := cReps[0]
	assert.Equal(t, cRepIn.Id, cRepOut.Id)
	assert.Equal(t, cRepIn.Bandwidth, cRepOut.Bandwidth)
	assert.Equal(t, cRepIn.AccountId.Name, cRepOut.AccountId.Name)
	assert.Equal(t, cRepIn.Port, cRepOut.Port)
	if err := DeleteConnReply(u.Account, cRepIn.RequestId); err != nil {
		t.Error("Failed to delete the test connection reply.", err)
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}
}
