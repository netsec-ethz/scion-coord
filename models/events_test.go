package models

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestJoinRequest(t *testing.T) {
	jReqIn := &JoinRequest{IsdAs: "100-100", SigKey: "sigkey", EncKey: "enckey", Status: PENDING}
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
	jRepIn := &JoinReply{RequestId: 1234, JoiningIsdAs: "123-123", SigningIsdAs: "321-312",
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
	cReqIn := &ConnRequest{IsdAsToConnect: "111-111", RequesterIsdAs: "222-222",
		RequesterCertificate: "test_cert", Info: "test_info", IP: "123.123.123.123",
		Port: 555, OverlayType: "UPD/IP", MTU: 1472, Bandwidth: 1000, Signature: "test_sig",
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
	cRepIn := &ConnReply{RequestId: 4321, ReplyingIsdAs: "321-321", RequesterIsdAs: "678-678",
		Certificate: "test_cert", IP: "123.123.123.123", Port: 333, OverlayType: "UDP/IP",
		MTU: 1472, Bandwidth: 1000}
	if err := cRepIn.Insert(); err != nil {
		t.Error("Failed to insert the test connection reply.", err)
	}
	cReps, err := FindConnRepliesByIsdAs(cRepIn.RequesterIsdAs)
	if err != nil {
		t.Error("Failed to retrieve the test connection reply.", err)
	}
	assert.Equal(t, 1, len(cReps))
	assert.True(t, reflect.DeepEqual(cRepIn, &cReps[0]))
	if err := DeleteConnReplyById(cReps[0].RequestId); err != nil {
		t.Error("Failed to delete the test connection reply.", err)
	}

}
