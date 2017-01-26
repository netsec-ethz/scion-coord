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
	Id        uint64 `orm:"column(id);auto;pk"`
	RequestId uint64
	Info      string // free form text for the reply
	IsdToJoin uint64
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

func (jr *JoinRequest) Insert() error {
	_, err := o.Insert(jr)
	return err
}

func (jr *JoinRequest) Update() error {
	_, err := o.Update(jr)
	return err
}

func FindJoinRequest(acc *Account, req_id uint64) (*JoinRequest, error) {
	req := new(JoinRequest)
	err := o.QueryTable(req).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(req)
	return req, err
}

func FindConnRequest(acc *Account, req_id uint64) (*ConnRequest, error) {
	req := new(ConnRequest)
	err := o.QueryTable(req).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(req)
	return req, err
}

func DeleteJoinRequest(acc *Account, req_id uint64) error {
	// orm beego can only delete with the primary key
	req := new(JoinRequest)
	err := o.QueryTable(req).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(req)
	if err != nil {
		return err
	}
	_, err = o.Delete(req)
	return err
}

type JoinReply struct {
	Id          uint64 `orm:"column(id);auto;pk"`
	RequestId   uint64
	Info        string   // free form text for the reply
	Account     *Account `orm:"rel(fk)"`
	Status      string
	JoiningIA   string
	IsCore      string // whether the new AS joins as core
	RespondIA   string
	Certificate string `orm:"type(text)"`
	TRC         string `orm:"type(text)"`
}

func FindJoinReply(acc *Account, req_id uint64) (*JoinReply, error) {
	jr := new(JoinReply)
	err := o.QueryTable(jr).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(jr)
	return jr, err
}

func (jr *JoinReply) Insert() error {
	_, err := o.Insert(jr)
	return err
}

func DeleteJoinReply(acc *Account, req_id uint64) error {
	// orm beego can only delete with the primary key
	rep := new(JoinReply)
	err := o.QueryTable(rep).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(rep)
	if err != nil {
		return err
	}
	_, err = o.Delete(rep)
	return err
}

type ConnRequest struct {
	Id                   uint64 `orm:"column(id);auto;pk"`
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

func FindConnRequestsByRespondIA(isdas string) ([]ConnRequest, error) {
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

func DeleteConnRequest(acc *Account, req_id uint64) error {
	// orm beego can only delete with the primary key
	req := new(ConnRequest)
	err := o.QueryTable(req).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(req)
	if err != nil {
		return err
	}
	_, err = o.Delete(req)
	return err
}

type ConnReply struct {
	Id          uint64 `orm:"column(id);auto;pk"`
	RequestId   uint64
	Info        string   // free form text for the reply
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

func FindConnRepliesByRequestIA(isdas string) ([]ConnReply, error) {
	var cr []ConnReply
	_, err := o.QueryTable("conn_reply").Filter("RequestIA", isdas).RelatedSel().All(&cr)
	return cr, err
}

func (cr *ConnReply) Insert() error {
	_, err := o.Insert(cr)
	return err
}

func DeleteConnReply(acc *Account, req_id uint64) error {
	// orm beego can only delete with the primary key
	rep := new(ConnReply)
	err := o.QueryTable(rep).Filter("Account", acc).Filter("RequestId", req_id).RelatedSel().One(rep)
	if err != nil {
		return err
	}
	_, err = o.Delete(rep)
	return err
}
