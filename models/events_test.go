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
	"fmt"
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
	jReqIn := &JoinRequest{Id: 1, RequestId: 777, RespondIA: "100-100", SigKey: "sigkey",
		Account: u.Account, EncKey: "enckey", Status: PENDING}
	if err := jReqIn.Insert(); err != nil {
		t.Error("Failed to insert the test join request.", err)
	}
	jReqOut, err := FindJoinRequestByRequestId(jReqIn.RequestId)
	if err != nil {
		t.Error("Failed to retrieve the test join request.", err)
	}
	assert.Equal(t, jReqIn.RequestId, jReqOut.RequestId)
	assert.Equal(t, jReqIn.RespondIA, jReqOut.RespondIA)
	assert.Equal(t, jReqIn.Account.Name, jReqOut.Account.Name)
	assert.Equal(t, jReqIn.SigKey, jReqOut.SigKey)
	assert.Equal(t, jReqIn.EncKey, jReqOut.EncKey)
	assert.Equal(t, jReqIn.Status, jReqOut.Status)
	if err := jReqIn.Delete(); err != nil {
		t.Error("Failed to delete the test join request.", err)
	}
	if err := u.Delete(); err != nil {
		t.Error(err)
	}
}

func TestJoinReply(t *testing.T) {
	u := createTestUser(t)
	jRepIn := &JoinReply{Id: 4, RequestId: 1234, JoiningIA: "123-123", RespondIA: "321-312",
		Account: u.Account, Certificate: "cert", TRC: "trc"}
	if err := jRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test join reply.", err)
	}
	jRepOut, err := FindJoinReplyByRequestId(jRepIn.RequestId)
	fmt.Printf("Id: %v", jRepOut)
	if err != nil {
		t.Error("Failed to retrieve the test join reply.", err)
	}
	assert.Equal(t, jRepIn.Id, jRepOut.Id)
	assert.Equal(t, jRepIn.JoiningIA, jRepOut.JoiningIA)
	assert.Equal(t, jRepIn.RespondIA, jRepOut.RespondIA)
	assert.Equal(t, jRepIn.Account.Name, jRepOut.Account.Name)
	assert.Equal(t, jRepIn.Certificate, jRepOut.Certificate)
	assert.Equal(t, jRepIn.TRC, jRepOut.TRC)
	if err := jRepIn.Delete(); err != nil {
		t.Error("Failed to delete the test join reply.", err)
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestConnRequest(t *testing.T) {
	u := createTestUser(t)
	cReqIn := &ConnRequest{Id: 5, RequestId: 8, RespondIA: "111-111", RequestIA: "222-222",
		Account: u.Account, RequesterCertificate: "test_cert", Info: "test_info",
		IP: "123.123.123.123", Port: 555, OverlayType: "UDP/IP", MTU: 1472, Bandwidth: 1000,
		Signature: "test_sig", Status: PENDING}
	if err := cReqIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection request.", err)
	}
	cReqOut, err := FindConnRequestByRequestId(cReqIn.Id)
	if err != nil {
		t.Error("Failed to retrieve the test connection request.", err)
	}
	assert.Equal(t, cReqIn.RequestId, cReqOut.RequestId)
	assert.Equal(t, cReqIn.IP, cReqOut.IP)
	assert.Equal(t, cReqIn.Account.Name, cReqOut.Account.Name)
	assert.Equal(t, cReqIn.OverlayType, cReqOut.OverlayType)
	if err := cReqOut.Delete(); err != nil {
		t.Error("Failed to delete the test connection request.", err)
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}
}

func TestConnReply(t *testing.T) {
	u := createTestUser(t)
	cRepIn := &ConnReply{Id: 6, RequestId: 4321, RespondIA: "321-321", RequestIA: "678-678",
		Account: u.Account, Certificate: "test_cert", IP: "123.123.123.123", Port: 333,
		OverlayType: "UDP/IP", MTU: 1472, Bandwidth: 1000}
	if err := cRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection reply.", err)
	}
	cReps, err := FindConnRepliesByIsdAs(cRepIn.RequestIA)
	if err != nil {
		t.Error("Failed to retrieve the test connection reply.", err)
	}
	assert.Equal(t, 1, len(cReps))
	cRepOut := cReps[0]
	assert.Equal(t, cRepIn.Id, cRepOut.Id)
	assert.Equal(t, cRepIn.Bandwidth, cRepOut.Bandwidth)
	assert.Equal(t, cRepIn.Account.Name, cRepOut.Account.Name)
	assert.Equal(t, cRepIn.Port, cRepOut.Port)
	if err := cRepOut.Delete(); err != nil {
		t.Error("Failed to delete the test connection reply.", err)
	}
	if err := u.Delete(); err != nil {
		t.Fatal(err)
	}
}
