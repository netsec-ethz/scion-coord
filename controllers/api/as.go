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

package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/models"
	"log"
	"net/http"
	"strings"
	"time"
)

type ConnRequest struct {
	RequestId   uint64
	Info        string // free form text motivation for the request
	RequestIA   string
	RespondIA   string
	OverlayType string
	IP          string
	Port        uint64
	MTU         uint64 // bytes
	Bandwidth   uint64 // kbps
	LinkType    string
	Timestamp   string // UTC ISO 8601 format string, 1s precision
	Signature   string
	Certificate string // certificate of the requesting AS
}

type ConnReply struct {
	RequestId   uint64
	Status      string
	Info        string // free form text for the reply
	RequestIA   string
	RespondIA   string
	OverlayType string
	IP          string
	Port        uint64
	MTU         uint64 // bytes
	Bandwidth   uint64 // kbps
	Certificate string // certificate of the responding AS
}

type JoinRequest struct {
	RequestId uint64
	Info      string // free form text motivation for the request
	IsdToJoin uint64
	SigKey    string // signing public key
	EncKey    string // encryption public key
}

type JoinReply struct {
	RequestId   uint64
	Status      string
	Info        string // free form text for the reply
	JoiningIA   string
	IsCore      string // whether the new AS joins as core
	RespondIA   string
	Certificate string `orm:"type(text)"` // certificate generated for the newly joining AS
	TRC         string `orm:"type(text)"`
}

func FindAccountByRequest(r *http.Request) (*models.Account, error) {
	key := mux.Vars(r)["key"]

	// get the account from the key and secret
	if key == "" {
		key = r.URL.Query().Get("key")
	}

	// find the account belonging to the request
	return models.FindAccountByKey(key)
}

func ValidateAccountOwnsIsdAs(account *models.Account, isdas string) (bool, error) {
	as, err := models.FindAsByIsdAs(isdas)
	if err != nil {
		return false, err
	}
	return as.Account.Id == account.Id, nil
}

type ASController struct {
	controllers.HTTPController
}

func (c *ASController) Exists(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isdas := vars["isdas"]

	if isdas == "" {
		c.BadRequest(errors.New("missing isdas parameter"), w, r)
		return
	}

	if _, err := models.FindAsByIsdAs(isdas); err != nil {
		c.NotFound(errors.New(isdas+" not found"), w, r)
		return
	}

	fmt.Fprintln(w, "{}")
}

func (c *ASController) Insert(w http.ResponseWriter, r *http.Request) {
	var as struct {
		IsdAs string `json:"isdas"`
		Core  bool   `json:"core"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&as); err != nil {
		c.BadRequest(err, w, r)
		return
	}

	// get the account from the key and secret
	account, err := FindAccountByRequest(r)
	if err != nil {
		c.Error500(err, w, r)
		return
	}

	isd, err := models.IsdAsToIsd(as.IsdAs)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	// create a new DB model
	finalAs := models.As{
		IsdAs:   as.IsdAs,
		Isd:     isd,
		Core:    as.Core,
		Account: account,
		Created: time.Now().UTC(),
	}

	// upsert it
	if err := finalAs.Insert(); err != nil {
		c.Error500(err, w, r)
		return
	}

	// we can decide whether to return the data back or a simple 200
	fmt.Fprintln(w, "{}")
}

// Find the account using the 'key' in the request
// and ensure the account owns the concerned ISD-AS
func (c *ASController) findAndValidateAccount(w http.ResponseWriter, r *http.Request,
	isdas string) (*models.Account, error) {

	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account. Key: %v, Request: %v: %v", mux.Vars(r)["key"], r, err)
		c.BadRequest(err, w, r)
		return nil, err
	}
	owns, err := ValidateAccountOwnsIsdAs(account, isdas)
	if err != nil {
		log.Printf("Error validating account %v owns ISD-AS %v: %v", account, isdas, err)
		c.Error500(err, w, r)
		return nil, err
	}
	if !owns {
		log.Printf("Account %v and AS %v do not match.", account, isdas)
		c.BadRequest(fmt.Errorf("Account %v and AS %v do not match.", account, isdas), w, r)
		return nil, err
	}
	return account, nil
}

func (c *ASController) UploadJoinRequest(w http.ResponseWriter, r *http.Request) {
	var request JoinRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	// find the account belonging to the request
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account for request: %v: %v", request, err)
		c.Error500(err, w, r)
		return
	}
	isd_to_join := request.IsdToJoin
	// find core AS in the ISD to join
	core_ases, err := models.FindCoreASesByIsd(isd_to_join)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	if len(core_ases) == 0 {
		log.Printf("ISD %v not found or no core ASes exist for this ISD. Account: %v",
			isd_to_join, account)
		c.Error500(err, w, r)
		return
	}
	// TODO(ercanucan): Send the request to ALL core ASes in this ISD.
	core_as := core_ases[0]
	join_request := models.JoinRequest{
		RequestId: request.RequestId,
		Info:      request.Info,
		IsdToJoin: request.IsdToJoin,
		Account:   account,
		RespondIA: core_as.IsdAs,
		SigKey:    request.SigKey,
		EncKey:    request.EncKey,
		Status:    models.PENDING,
	}
	// insert into the join_requests table in the database
	if err := join_request.Insert(); err != nil {
		log.Printf("Error inserting join request for core AS %v: %v", core_as, err)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, "{}")
}

func (c *ASController) UploadJoinReply(w http.ResponseWriter, r *http.Request) {
	var reply JoinReply
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reply); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, reply.JoiningIA)
	if err != nil {
		return
	}
	joinReply := models.JoinReply{
		RequestId:   reply.RequestId,
		Account:     account,
		Status:      reply.Status,
		JoiningIA:   reply.JoiningIA,
		IsCore:      reply.IsCore,
		RespondIA:   reply.RespondIA,
		Certificate: reply.Certificate,
		TRC:         reply.TRC,
	}
	if err := joinReply.Insert(); err != nil {
		log.Printf("Error inserting join reply. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, reply.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	// Change the join req's status to approved/rejected.
	join_req, err := models.FindJoinRequest(account, joinReply.RequestId)
	if err != nil {
		log.Printf("Error finding join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, reply.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	join_req.Status = reply.Status
	if err := join_req.Update(); err != nil {
		log.Printf("Error updating join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, reply.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Received a join reply. Account: %v Request ID: %v ISD-AS: %v Status: %v",
		account, joinReply.RequestId, reply.RespondIA, reply.Status)
	if reply.Status == models.APPROVED {
		isd, err := models.IsdAsToIsd(joinReply.JoiningIA)
		if err != nil {
			log.Printf("Error extracting ISD from ISD-AS %v, %v ", joinReply.JoiningIA, err)
			c.Error500(err, w, r)
			return
		}
		var is_core bool
		if strings.ToLower(joinReply.IsCore) == "true" {
			is_core = true
		} else {
			is_core = false
		}
		new_as := models.As{
			IsdAs:   joinReply.JoiningIA,
			Isd:     isd,
			Core:    is_core,
			Account: account,
			Created: time.Now().UTC(),
		}
		if new_as.Insert(); err != nil {
			log.Printf("Error inserting new AS: %v Account: %v Request ID: %v, %v",
				new_as.IsdAs, account, reply.RequestId, err)
			c.Error500(err, w, r)
			return
		}
		log.Printf("New AS successfully created. Account: %v Request ID: %v new AS: %v",
			account, reply.RequestId, reply.JoiningIA)
	}
	fmt.Fprintln(w, "{}")
}

func (c *ASController) PollJoinReply(w http.ResponseWriter, r *http.Request) {
	var request struct {
		RequestId uint64 `json:"request_id"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account for request: %v: %v", request.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	joinReply, err := models.FindJoinReply(account, request.RequestId)
	if err != nil {
		log.Printf("Join reply not found. Account: %v Request ID: %v, %v",
			account, request.RequestId, err)
		c.NotFound(err, w, r)
		return
	}
	reply := JoinReply{
		RequestId:   joinReply.RequestId,
		Status:      joinReply.Status,
		RespondIA:   joinReply.RespondIA,
		IsCore:      joinReply.IsCore,
		JoiningIA:   joinReply.JoiningIA,
		Certificate: joinReply.Certificate,
		TRC:         joinReply.TRC,
	}
	b, err := json.Marshal(reply)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v new AS: %v, %v",
			account, request.RequestId, joinReply.JoiningIA, err)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnRequest(w http.ResponseWriter, r *http.Request) {
	var cr ConnRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&cr); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, cr.RequestIA)
	if err != nil {
		return
	}
	connRequest := models.ConnRequest{
		RequestId:            cr.RequestId,
		Account:              account,
		RequestIA:            cr.RequestIA,
		RespondIA:            cr.RespondIA,
		Info:                 cr.Info,
		IP:                   cr.IP,
		Port:                 cr.Port,
		OverlayType:          cr.OverlayType,
		MTU:                  cr.MTU,
		Bandwidth:            cr.Bandwidth,
		LinkType:             cr.LinkType,
		Timestamp:            cr.Timestamp,
		Signature:            cr.Signature,
		RequesterCertificate: cr.Certificate,
		Status:               models.PENDING,
	}
	if err := connRequest.Insert(); err != nil {
		log.Printf("Error inserting Connection Request. Account %v AS %v: %v", account,
			cr.RequestIA, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Connection Request Successfully Received: %v Request ID: %v",
		account, cr.RequestId)
	fmt.Fprintln(w, "{}")
}

func (c *ASController) UploadConnReply(w http.ResponseWriter, r *http.Request) {
	var reply ConnReply
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reply); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, reply.RequestIA)
	if err != nil {
		return
	}
	connReply := models.ConnReply{
		RequestId:   reply.RequestId,
		Account:     account,
		Status:      reply.Status,
		RespondIA:   reply.RespondIA,
		RequestIA:   reply.RequestIA,
		Certificate: reply.Certificate, // Certificate of the replying AS.
		IP:          reply.IP,
		Port:        reply.Port,
		OverlayType: reply.OverlayType,
		MTU:         reply.MTU,
		Bandwidth:   reply.Bandwidth,
	}
	if err := connReply.Insert(); err != nil {
		log.Printf("Error inserting Connection Reply. Request ID: %v Account: %v AS: %v: %v",
			reply.RequestId, account, reply.RespondIA, err)
		c.BadRequest(err, w, r)
		return
	}
	// Change the conn req's status to approved/rejected.
	cr, err := models.FindConnRequest(account, reply.RequestId)
	if err != nil {
		log.Printf("Error finding conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, reply.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	cr.Status = reply.Status
	if err := cr.Update(); err != nil {
		log.Printf("Error updating conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, reply.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Connection Reply Successfully Received. Account: %v Request ID: %v "+
		"Requesting AS: %v Replying AS: %v Status: %v", account, reply.RequestId, reply.RequestIA,
		reply.RespondIA, reply.Status)
	fmt.Fprintln(w, "{}")
}

// Converts the DB layer JoinRequest array to API layer JoinRequest array
func (c *ASController) prepJoinRequests(in []models.JoinRequest) []JoinRequest {
	if in == nil || len(in) == 0 {
		return make([]JoinRequest, 0)
	}
	out := make([]JoinRequest, len(in))
	for i, v := range in {
		out[i] = JoinRequest{
			RequestId: v.RequestId,
			Info:      v.Info,
			IsdToJoin: v.IsdToJoin,
			SigKey:    v.SigKey,
			EncKey:    v.EncKey,
		}
	}
	return out
}

// Converts the DB layer ConnRequest array to API layer ConnRequest array
func (c *ASController) prepConnRequests(in []models.ConnRequest) []ConnRequest {
	if in == nil || len(in) == 0 {
		return make([]ConnRequest, 0)
	}
	out := make([]ConnRequest, len(in))
	for i, v := range in {
		out[i] = ConnRequest{
			RequestId:   v.RequestId,
			Info:        v.Info,
			RequestIA:   v.RequestIA,
			RespondIA:   v.RespondIA,
			OverlayType: v.OverlayType,
			IP:          v.IP,
			Port:        v.Port,
			MTU:         v.MTU,
			Bandwidth:   v.Bandwidth,
			LinkType:    v.LinkType,
			Timestamp:   v.Timestamp,
			Signature:   v.Signature,
			Certificate: v.RequesterCertificate,
		}
	}
	return out
}

// Converts the DB layer ConnReply array to API layer ConnReply array
func (c *ASController) prepConnReplies(in []models.ConnReply) []ConnReply {
	if in == nil || len(in) == 0 {
		return make([]ConnReply, 0)
	}
	out := make([]ConnReply, len(in))
	for i, v := range in {
		out[i] = ConnReply{
			RequestId:   v.RequestId,
			Status:      v.Status,
			Info:        v.Info,
			RequestIA:   v.RequestIA,
			RespondIA:   v.RespondIA,
			OverlayType: v.OverlayType,
			IP:          v.IP,
			Port:        v.Port,
			MTU:         v.MTU,
			Bandwidth:   v.Bandwidth,
			Certificate: v.Certificate,
		}
	}
	return out
}

// API end-point to query outstanding requests/events for an AS
func (c *ASController) PollEvents(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs string
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	isdas := req.IsdAs
	account, err := c.findAndValidateAccount(w, r, isdas)
	if err != nil {
		return
	}
	joinRequests, err := models.FindOpenJoinRequestsByIsdAs(isdas)
	if err != nil {
		log.Printf("Error while retrieving open join requests. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(err, w, r)
		return
	}
	connRequests, err := models.FindConnRequestsByRespondIA(isdas)
	if err != nil {
		log.Printf("Error while retrieving connection requests. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(err, w, r)
		return
	}
	connReplies, err := models.FindConnRepliesByRequestIA(isdas)
	if err != nil {
		log.Printf("Error while retrieving connection replies. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(err, w, r)
		return
	}

	var resp struct {
		JoinRequests []JoinRequest
		ConnRequests []ConnRequest
		ConnReplies  []ConnReply
	}
	resp.JoinRequests = c.prepJoinRequests(joinRequests)
	resp.ConnRequests = c.prepConnRequests(connRequests)
	resp.ConnReplies = c.prepConnReplies(connReplies)

	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling. Account: %v, ISD-AS: %v, %v", account, isdas,
			err)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}
