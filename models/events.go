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

	"github.com/astaxie/beego/orm"
	"github.com/scionproto/scion/go/lib/addr"
)

const (
	Pending  = "PENDING"
	Approved = "APPROVED"
)

type JoinRequest struct {
	ID            uint64   `orm:"column(id);auto;pk"`
	RequestID     uint64   `orm:"column(request_id)"`
	Info          string   // free form text for the reply
	ISDToJoin     addr.ISD `orm:"column(isd_to_join)"`       // the ISD that the sender wants to join
	JoinAsACoreAS bool     `orm:"column(join_as_a_core_as)"` // whether to join the ISD as a core AS
	RequesterID   string   `orm:"column(requester_id)"`      // the key to identify which account made the request
	RespondIA     string   `orm:"column(respond_ia)"`        // the ISD-AS which should respond to the request
	SigPubKey     string   // signing public key
	EncPubKey     string   // encryption public key
	Status        string
}

func FindOpenJoinRequestsByIA(isdas string) ([]JoinRequest, error) {
	var requests []JoinRequest
	_, err := o.QueryTable(new(JoinRequest)).Filter("RespondIA", isdas).Filter("Status",
		Pending).All(&requests)
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

func FindJoinRequest(requester string, reqID uint64) (*JoinRequest, error) {
	req := new(JoinRequest)
	err := o.QueryTable(req).Filter("RequesterID", requester).Filter("RequestID", reqID).RelatedSel().One(req)
	return req, err
}

func FindConnRequest(acc *Account, reqID uint64) (*ConnRequest, error) {
	req := new(ConnRequest)
	err := o.QueryTable(req).Filter("Account", acc).Filter("RequestID", reqID).RelatedSel().One(req)
	return req, err
}

func DeleteJoinRequest(requester string, reqID uint64) error {
	// orm beego can only delete with the primary key
	req := new(JoinRequest)
	err := o.QueryTable(req).Filter("RequesterID", requester).Filter("RequestID", reqID).RelatedSel().One(req)
	if err != nil {
		return err
	}
	_, err = o.Delete(req)
	return err
}

type JoinReply struct {
	ID                   uint64 `orm:"column(id);auto;pk"`
	RequestID            uint64 `orm:"column(request_id)"`
	Info                 string // free form text for the reply
	RequesterID          string `orm:"column(requester_id)"` // the string to identify which account made the request
	Status               string
	JoiningIA            string `orm:"column(joining_ia)"`
	IsCore               bool   // whether the new AS joins as core
	RespondIA            string `orm:"column(respond_ia)"`
	JoiningIACertificate string `orm:"column(joining_ia_certificate);type(text)"`    // certificate of the newly joining AS
	RespondIACertificate string `orm:"column(responding_ia_certificate);type(text)"` // certificate of the responding AS
	TRC                  string `orm:"column(trc);type(text)"`
}

func FindJoinReply(requester string, reqID uint64) (*JoinReply, error) {
	jr := new(JoinReply)
	err := o.QueryTable(jr).Filter("RequesterID", requester).Filter("RequestID", reqID).RelatedSel().One(jr)
	return jr, err
}

func (jr *JoinReply) Insert() error {
	existingJR := new(JoinReply)
	// should always return with orm.ErrNoRows
	err := o.QueryTable(jr).Filter("RequesterID", jr.RequesterID).Filter("RequestID", jr.RequestID).RelatedSel().One(existingJR)
	if err == nil {
		return fmt.Errorf("join reply already exists for this request")
	} else if err != orm.ErrNoRows { // some other error occurred during lookup
		return err
	}
	_, err = o.Insert(jr)
	return err
}

func DeleteJoinReply(requester string, reqID uint64) error {
	// orm beego can only delete with the primary key
	rep := new(JoinReply)
	err := o.QueryTable(rep).Filter("RequesterID", requester).Filter("RequestID", reqID).RelatedSel().One(rep)
	if err != nil {
		return err
	}
	_, err = o.Delete(rep)
	return err
}

type ConnRequest struct {
	ID                   uint64   `orm:"column(id);auto;pk"`
	RequestID            uint64   `orm:"column(request_id)"`
	Account              *Account `orm:"rel(fk)"` // account sending the connection request
	Status               string
	RequestIA            string `orm:"column(request_ia)"`
	RespondIA            string `orm:"column(respond_ia)"`
	RequesterCertificate string `orm:"type(text)"`
	Info                 string // free form text motivation for the request
	OverlayType          string
	IP                   string `orm:"column(ip)"`
	Port                 uint64
	MTU                  uint64 `orm:"column(mtu)"` // bytes
	Bandwidth            uint64 // kbps
	LinkType             string
	Timestamp            string // UTC ISO 8601 format string, 1s precision
	Signature            string
}

func FindOpenConnRequestsByRespondIA(isdas string) ([]ConnRequest, error) {
	var requests []ConnRequest
	_, err := o.QueryTable(new(ConnRequest)).Filter("RespondIA", isdas).Filter("Status",
		Pending).All(&requests)
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

func DeleteConnRequest(acc *Account, reqID uint64) error {
	// orm beego can only delete with the primary key
	req := new(ConnRequest)
	err := o.QueryTable(req).Filter("Account", acc).Filter("RequestID", reqID).RelatedSel().One(req)
	if err != nil {
		return err
	}
	_, err = o.Delete(req)
	return err
}

type ConnReply struct {
	ID          uint64   `orm:"column(id);auto;pk"`
	RequestID   uint64   `orm:"column(request_id)"`
	Info        string   // free form text for the reply
	Account     *Account `orm:"rel(fk)"` // account which should receive the connection reply
	Status      string
	RespondIA   string `orm:"column(respond_ia)"`
	RequestIA   string `orm:"column(request_ia)"`
	Certificate string `orm:"type(text)"`
	OverlayType string
	IP          string `orm:"column(ip)"`
	Port        uint64
	MTU         uint64 `orm:"column(mtu)"` // bytes
	Bandwidth   uint64 // kbps
}

func FindConnRepliesByRequestIA(isdas string) ([]ConnReply, error) {
	var cr []ConnReply
	_, err := o.QueryTable(new(ConnReply)).Filter("RequestIA", isdas).RelatedSel().All(&cr)
	return cr, err
}

func (cr *ConnReply) Insert() error {
	existingCR := new(ConnReply)
	// should always return with orm.ErrNoRows
	err := o.QueryTable(cr).Filter("Account", cr.Account).Filter("RequestID", cr.RequestID).RelatedSel().One(existingCR)
	if err == nil {
		return fmt.Errorf("connection reply already exists for this request")
	} else if err != orm.ErrNoRows { // means some other error occurred during lookup
		return err
	}
	_, err = o.Insert(cr)
	return err
}

func DeleteConnReply(acc *Account, reqID uint64) error {
	// orm beego can only delete with the primary key
	rep := new(ConnReply)
	err := o.QueryTable(rep).Filter("Account", acc).Filter("RequestID", reqID).RelatedSel().One(rep)
	if err != nil {
		return err
	}
	_, err = o.Delete(rep)
	return err
}
