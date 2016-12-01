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
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestJoinRequest(t *testing.T) {
	jReqIn := &JoinRequest{RespondIA: "100-100", SigKey: "sigkey", EncKey: "enckey", Status: PENDING}
	if err := jReqIn.Insert(); err != nil {
		t.Error("Failed to insert the test join request.", err)
	}
	jReqOut, err := FindJoinRequestByRequestId(jReqIn.Id)
	if err != nil {
		t.Error("Failed to retrieve the test join request.", err)
	}
	assert.True(t, reflect.DeepEqual(jReqIn, jReqOut))
	if err := DeleteJoinRequestById(jReqOut.Id); err != nil {
		t.Error("Failed to delete the test join request.", err)
	}
}

func TestJoinReply(t *testing.T) {
	jRepIn := &JoinReply{RequestId: 1234, JoiningIA: "123-123", RespondIA: "321-312",
		Certificate: "cert", TRC: "trc"}
	if err := jRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test join reply.", err)
	}
	jRepOut, err := FindJoinReplyByRequestId(jRepIn.RequestId)
	if err != nil {
		t.Error("Failed to retrieve the test join reply.", err)
	}
	assert.True(t, reflect.DeepEqual(jRepIn, jRepOut))
	if err := DeleteJoinReplyById(jRepOut.RequestId); err != nil {
		t.Error("Failed to delete the test join reply.", err)
	}
}

func TestConnRequest(t *testing.T) {
	cReqIn := &ConnRequest{RespondIA: "111-111", RequestIA: "222-222",
		RequesterCertificate: "test_cert", Info: "test_info", IP: "123.123.123.123",
		Port: 555, OverlayType: "UDP/IP", MTU: 1472, Bandwidth: 1000, Signature: "test_sig",
		Status: PENDING}
	if err := cReqIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection request.", err)
	}
	cReqOut, err := FindConnRequestByRequestId(cReqIn.Id)
	if err != nil {
		t.Error("Failed to retrieve the test connection request.", err)
	}
	assert.True(t, reflect.DeepEqual(cReqIn, cReqOut))
	if err := DeleteConnRequestById(cReqOut.Id); err != nil {
		t.Error("Failed to delete the test connection request.", err)
	}
}

func TestConnReply(t *testing.T) {
	cRepIn := &ConnReply{RequestId: 4321, RespondIA: "321-321", RequestIA: "678-678",
		Certificate: "test_cert", IP: "123.123.123.123", Port: 333, OverlayType: "UDP/IP",
		MTU: 1472, Bandwidth: 1000}
	if err := cRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection reply.", err)
	}
	cReps, err := FindConnRepliesByIsdAs(cRepIn.RequestIA)
	if err != nil {
		t.Error("Failed to retrieve the test connection reply.", err)
	}
	assert.Equal(t, 1, len(cReps))
	assert.True(t, reflect.DeepEqual(cRepIn, &cReps[0]))
	if err := DeleteConnReplyById(cReps[0].RequestId); err != nil {
		t.Error("Failed to delete the test connection reply.", err)
	}
}
