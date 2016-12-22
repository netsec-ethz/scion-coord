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
	Id        uint64 `orm:"pk"`
	RequestId uint64
	Account   *Account `orm:"rel(fk)"`
	RespondIA string
	SigKey    string
	EncKey    string
	Status    string
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

func DeleteJoinRequestById(id uint64) error {
	_, err := o.Delete(&JoinRequest{Id: id})
	return err
}

type JoinReply struct {
	Id          uint64 `orm:"pk"`
	RequestId   uint64
	Account     *Account `orm:"rel(fk)"`
	Status      string
	JoiningIA   string
	RespondIA   string
	Certificate string `orm:"type(text)"`
	TRC         string `orm:"type(text)"`
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

func DeleteJoinReplyById(id uint64) error {
	_, err := o.Delete(&JoinReply{Id: id})
	return err
}

type ConnRequest struct {
	Id                   uint64 `orm:"pk"`
	RequestId            uint64
	Account              *Account `orm:"rel(fk)"`
	Status               string
	RequestIA            string
	RespondIA            string
	RequesterCertificate string `orm:"type(text)"`
	Info                 string // free form text motivation for the request
	OverlayType          string
	IP                   string
	Port                 uint64
	MTU                  uint64 // bytes
	Bandwidth            uint64 // kbps
	LinkType             string
	Timestamp            string // UTC ISO 8601 format string, 1s precision
	Signature            string
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

func (cr *ConnRequest) Delete() error {
	_, err := o.Delete(cr)
	return err
}

func DeleteConnRequestById(id uint64) error {
	_, err := o.Delete(&ConnRequest{Id: id})
	return err
}

type ConnReply struct {
	Id          uint64 `orm:"pk"`
	RequestId   uint64
	Account     *Account `orm:"rel(fk)"`
	Status      string
	RespondIA   string
	RequestIA   string
	Certificate string `orm:"type(text)"`
	OverlayType string
	IP          string
	Port        uint64
	MTU         uint64 // bytes
	Bandwidth   uint64 // kbps
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

func (cr *ConnReply) Delete() error {
	_, err := o.Delete(cr)
	return err
}

func DeleteConnReplyById(id uint64) error {
	_, err := o.Delete(&ConnReply{Id: id})
	return err
}
