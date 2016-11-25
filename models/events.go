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

const (
	PENDING  = "PENDING"
	APPROVED = "APPROVED"
)

type JoinRequest struct {
	Id     uint64 `json:"id"`
	IsdAs  string `json:"isdas"`
	SigKey string `json:"sigkey"`
	EncKey string `json:"enckey"`
	Status string `json:"status"`
}

func FindOpenJoinRequestsByIsdAs(isdas string) ([]JoinRequest, error) {
	var requests []JoinRequest
	_, err := o.QueryTable("join_request").Filter("IsdAs", isdas).Filter("Status", PENDING).All(&requests)
	return requests, err
}

func FindJoinRequestsByIsdAs(isdas string) ([]JoinRequest, error) {
	var requests []JoinRequest
	_, err := o.QueryTable("join_request").Filter("IsdAs", isdas).All(&requests)
	return requests, err
}

func (jr *JoinRequest) Insert() error {
	_, err := o.Insert(jr)
	return err
}

func (jr *JoinRequest) Update() error {
	_, err := o.Update(jr)
	return err
}

func (jr *JoinRequest) Delete() error {
	_, err := o.Delete(jr)
	return err
}

func FindJoinRequestByRequestId(id uint64) (*JoinRequest, error) {
	req := new(JoinRequest)
	err := o.QueryTable(req).Filter("Id", id).RelatedSel().One(req)
	return req, err
}

func FindConnRequestByRequestId(id uint64) (*ConnRequest, error) {
	req := new(ConnRequest)
	err := o.QueryTable(req).Filter("Id", id).RelatedSel().One(req)
	return req, err
}

func DeleteJoinRequestById(id uint64) error {
	_, err := o.Delete(&JoinRequest{Id: id})
	return err
}

type JoinRequestMapping struct {
	Id      uint64   `json:"id"`
	IsdAs   string   `json:"isdas"`
	Account *Account `orm:"rel(fk)"`
}

func FindJoinMappingByRequestId(id uint64) (*JoinRequestMapping, error) {
	jrm := new(JoinRequestMapping)
	err := o.QueryTable(jrm).Filter("Id", id).RelatedSel().One(jrm)
	return jrm, err
}

func (jrm *JoinRequestMapping) Insert() error {
	_, err := o.Insert(jrm)
	return err
}

func (jrm *JoinRequestMapping) Delete() error {
	_, err := o.Delete(jrm)
	return err
}

type JoinReply struct {
	RequestId    uint64 `json:"request_id" orm:"pk"`
	JoiningIsdAs string `json:"joining_isdas"`
	SigningIsdAs string `json:"signing_isdas"`
	Certificate  string `json:"certificate" orm:"type(text)"`
	TRC          string `json:"trc" orm:"type(text)"`
}

func FindJoinReplyByRequestId(id uint64) (*JoinReply, error) {
	jr := new(JoinReply)
	err := o.QueryTable(jr).Filter("RequestId", id).RelatedSel().One(jr)
	return jr, err
}

func (jr *JoinReply) Insert() error {
	_, err := o.Insert(jr)
	return err
}

func (jr *JoinReply) Delete() error {
	_, err := o.Delete(jr)
	return err
}

func DeleteJoinReplyById(id uint64) error {
	_, err := o.Delete(&JoinReply{RequestId: id})
	return err
}

type ConnRequest struct {
	Id                   uint64 `json:"id"`
	IsdAsToConnectTo     string `json:"isdas_to_connect"`
	RequesterIsdAs       string `json:"requester_isdas"`
	RequesterCertificate string `json:"requester_certificate" orm:"type(text)"`
	Info                 string `json:"info"` // free form text motivation for the request
	OverlayType          string `json:"overlay_type"`
	IP                   string `json:"ip"`
	Port                 uint64 `json:"port"`
	MTU                  uint64 `json:"mtu"`       // bytes
	Bandwidth            uint64 `json:"bandwidth"` // kbps
	LinkType             string `json:"link_type"`
	Timestamp            string `json:"timestamp"` // UTC ISO 8601 format string
	Signature            string `json:"signature"`
	Status               string `json:"status"`
}

func FindConnRequestsByIsdAs(isdas string) ([]ConnRequest, error) {
	var requests []ConnRequest
	_, err := o.QueryTable("conn_request").Filter("IsdAsToConnectTo", isdas).All(&requests)
	return requests, err
}

func (cr *ConnRequest) Insert() error {
	_, err := o.Insert(cr)
	return err
}

func (cr *ConnRequest) Update() error {
	_, err := o.Update(cr)
	return err
}

func DeleteConnRequestById(id uint64) error {
	_, err := o.Delete(&ConnRequest{Id: id})
	return err
}

type ConnRequestMapping struct {
	RequestId        uint64 `orm:"pk"`
	RequesterIsdAs   string
	IsdAsToConnectTo string
}

func FindConnMappingByRequestId(id uint64) (ConnRequestMapping, error) {
	var crm ConnRequestMapping
	err := o.QueryTable(crm).Filter("RequestId", id).RelatedSel().One(&crm)
	return crm, err
}

func (crm *ConnRequestMapping) Insert() error {
	_, err := o.Insert(crm)
	return err
}

func DeleteConnMappingById(id uint64) error {
	_, err := o.Delete(&ConnRequestMapping{RequestId: id})
	return err
}

type ConnReply struct {
	RequestId      uint64 `json:"request_id" orm:"pk"`
	ReplyingIsdAs  string `json:"replying_isdas"`
	RequesterIsdAs string `json:"requester_isdas"`
	Certificate    string `json:"certificate" orm:"type(text)"`
	OverlayType    string `json:"overlay_type"`
	IP             string `json:"ip"`
	Port           uint64 `json:"port"`
	MTU            uint64 `json:"mtu"`       // bytes
	Bandwidth      uint64 `json:"bandwidth"` // kbps
}

func FindConnRepliesByIsdAs(isdas string) ([]ConnReply, error) {
	var cr []ConnReply
	_, err := o.QueryTable("conn_reply").Filter("RequesterIsdAs", isdas).RelatedSel().All(&cr)
	return cr, err
}

func (cr *ConnReply) Insert() error {
	_, err := o.Insert(cr)
	return err
}

func DeleteConnReplyById(id uint64) error {
	_, err := o.Delete(&ConnReply{RequestId: id})
	return err
}
