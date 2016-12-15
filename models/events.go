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
	Id        uint64   `json:"id" orm:"pk"`
	RequestId uint64   `json:"request_id"`
	Account   *Account `orm:"rel(fk)"`
	RespondIA string   `json:"respond_ia"`
	SigKey    string   `json:"sigkey"`
	EncKey    string   `json:"enckey"`
	Status    string   `json:"status"`
}

func FindOpenJoinRequestsByIsdAs(isdas string) ([]JoinRequest, error) {
	var requests []JoinRequest
	_, err := o.QueryTable("join_request").Filter("RespondIA", isdas).Filter("Status", PENDING).All(&requests)
	return requests, err
}

func FindJoinRequestsByIsdAs(isdas string) ([]JoinRequest, error) {
	var requests []JoinRequest
	_, err := o.QueryTable("join_request").Filter("RespondIA", isdas).All(&requests)
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

func FindJoinRequestByRequestId(req_id uint64) (*JoinRequest, error) {
	req := new(JoinRequest)
	err := o.QueryTable(req).Filter("RequestId", req_id).RelatedSel().One(req)
	return req, err
}

func FindConnRequestByRequestId(id uint64) (*ConnRequest, error) {
	req := new(ConnRequest)
	err := o.QueryTable(req).Filter("Id", id).RelatedSel().One(req)
	return req, err
}

func DeleteJoinRequestByRequestId(req_id uint64) error {
	_, err := o.Delete(&JoinRequest{RequestId: req_id})
	return err
}

type JoinReply struct {
	Id          uint64   `json:"id" orm:"pk"`
	RequestId   uint64   `json:"request_id"`
	Account     *Account `orm:"rel(fk)"`
	Status      string   `json:"status"`
	JoiningIA   string   `json:"joining_ia"`
	RespondIA   string   `json:"respond_ia"`
	Certificate string   `json:"certificate" orm:"type(text)"`
	TRC         string   `json:"trc" orm:"type(text)"`
}

func FindJoinReplyByRequestId(req_id uint64) (*JoinReply, error) {
	jr := new(JoinReply)
	err := o.QueryTable(jr).Filter("RequestId", req_id).RelatedSel().One(jr)
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

func DeleteJoinReplyById(req_id uint64) error {
	_, err := o.Delete(&JoinReply{RequestId: req_id})
	return err
}

type ConnRequest struct {
	Id                   uint64   `json:"id" orm:"pk"`
	RequestId            uint64   `json:"request_id"`
	Account              *Account `orm:"rel(fk)"`
	Status               string   `json:"status"`
	RequestIA            string   `json:"request_ia"`
	RespondIA            string   `json:"respond_ia"`
	RequesterCertificate string   `json:"requester_certificate" orm:"type(text)"`
	Info                 string   `json:"info"` // free form text motivation for the request
	OverlayType          string   `json:"overlay_type"`
	IP                   string   `json:"ip"`
	Port                 uint64   `json:"port"`
	MTU                  uint64   `json:"mtu"`       // bytes
	Bandwidth            uint64   `json:"bandwidth"` // kbps
	LinkType             string   `json:"link_type"`
	Timestamp            string   `json:"timestamp"` // UTC ISO 8601 format string, 1s precision
	Signature            string   `json:"signature"`
}

func FindConnRequestsByIsdAs(isdas string) ([]ConnRequest, error) {
	var requests []ConnRequest
	_, err := o.QueryTable("conn_request").Filter("RespondIA", isdas).All(&requests)
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

func DeleteConnRequestById(req_id uint64) error {
	_, err := o.Delete(&ConnRequest{RequestId: req_id})
	return err
}

type ConnReply struct {
	Id          uint64   `json:"id" orm:"pk"`
	RequestId   uint64   `json:"request_id"`
	Account     *Account `orm:"rel(fk)"`
	Status      string   `json:"status"`
	RespondIA   string   `json:"respond_ia"`
	RequestIA   string   `json:"request_ia"`
	Certificate string   `json:"certificate" orm:"type(text)"`
	OverlayType string   `json:"overlay_type"`
	IP          string   `json:"ip"`
	Port        uint64   `json:"port"`
	MTU         uint64   `json:"mtu"`       // bytes
	Bandwidth   uint64   `json:"bandwidth"` // kbps
}

func FindConnRepliesByIsdAs(isdas string) ([]ConnReply, error) {
	var cr []ConnReply
	_, err := o.QueryTable("conn_reply").Filter("RequestIA", isdas).RelatedSel().All(&cr)
	return cr, err
}

func (cr *ConnReply) Insert() error {
	_, err := o.Insert(cr)
	return err
}

func DeleteConnReplyById(req_id uint64) error {
	_, err := o.Delete(&ConnReply{RequestId: req_id})
	return err
}
