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
	"time"
)

type ConnRequest struct {
	RequestId   uint64 `json:"request_id"`
	Info        string `json:"info"` // free form text motivation for the request
	RespondIA   string `json:"respond_ia"`
	OverlayType string `json:"overlay_type"`
	IP          string `json:"ip"`
	Port        uint64 `json:"port"`
	MTU         uint64 `json:"mtu"`       // bytes
	Bandwidth   uint64 `json:"bandwidth"` // kbps
	LinkType    string `json:"link_type"`
	Timestamp   string `json:"timestamp"` // UTC ISO 8601 format string, 1s precision
	Signature   string `json:"signature"` // signature is over IsdAs and ConnRequest
	Certificate string `json:"certificate"`
}

type ConnReply struct {
	RequestId   uint64 `json:"request_id"`
	Status      string `json:"status"`
	RespondIA   string `json:"respond_ia"`
	RequestIA   string `json:"request_ia"`
	OverlayType string `json:"overlay_type"`
	IP          string `json:"ip"`
	Port        uint64 `json:"port"`
	MTU         uint64 `json:"mtu"`       // bytes
	Bandwidth   uint64 `json:"bandwidth"` // kbps
	Certificate string `json:"certificate"`
}

type JoinRequest struct {
	RequestId uint64 `json:"request_id"`
	IsdToJoin uint64 `json:"isd_to_join"`
	SigKey    string `json:"sigkey"`
	EncKey    string `json:"enckey"`
}

type JoinReply struct {
	RequestId   uint64 `json:"request_id"`
	Status      string `json:"status"`
	JoiningIA   string `json:"joining_ia"`
	RespondIA   string `json:"respond_ia"`
	Certificate string `json:"certificate" orm:"type(text)"`
	TRC         string `json:"trc" orm:"type(text)"`
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
		c.NotFound(isdas+" not found", w, r)
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
	// return join_request_id
	var resp struct {
		RequestId uint64 `json:"request_id"`
	}
	resp.RequestId = join_request.RequestId
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v core AS: %v ISD %v, %v",
			account, resp.RequestId, core_as.IsdAs, isd_to_join, err)
		fmt.Fprintln(w, err)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadJoinReply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RespondIA string    `json:"respond_ia"`
		JoinReply JoinReply `json:"join_reply"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	reply := req.JoinReply
	account, err := c.findAndValidateAccount(w, r, req.RespondIA)
	if err != nil {
		return
	}
	joinReply := models.JoinReply{
		RequestId:   reply.RequestId,
		Account:     account,
		Status:      reply.Status,
		JoiningIA:   reply.JoiningIA,
		RespondIA:   req.RespondIA,
		Certificate: reply.Certificate,
		TRC:         reply.TRC,
	}
	if err := joinReply.Insert(); err != nil {
		log.Printf("Error inserting join reply. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, req.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	// Change the join req's status to approved.
	join_req, err := models.FindJoinRequestByRequestId(joinReply.RequestId)
	if err != nil {
		log.Printf("Error finding join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, req.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	join_req.Status = reply.Status
	if err := join_req.Update(); err != nil {
		log.Printf("Error updating join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, req.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Join reply successfully received. Account: %v Request ID: %v ISD-AS: %v",
		account, joinReply.RequestId, req.RespondIA)
	if reply.Status == models.APPROVED {
		isd, err := models.IsdAsToIsd(joinReply.JoiningIA)
		if err != nil {
			log.Printf("Error extracting ISD from ISD-AS %v, %v ", joinReply.JoiningIA, err)
			c.Error500(err, w, r)
			return
		}
		new_as := models.As{
			IsdAs:   joinReply.JoiningIA,
			Isd:     isd,
			Core:    false,
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
	joinReply, err := models.FindJoinReplyByRequestId(request.RequestId)
	if err != nil {
		log.Printf("Error fetching join reply. Account: %v Request ID: %v, %v",
			account, request.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	reply := JoinReply{
		RequestId:   joinReply.RequestId,
		Status:      joinReply.Status,
		RespondIA:   joinReply.RespondIA,
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
	var req struct {
		RequestIA   string      `json:"request_ia"`
		ConnRequest ConnRequest `json:"request"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, req.RequestIA)
	if err != nil {
		return
	}
	cr := req.ConnRequest
	connRequest := models.ConnRequest{
		RequestId:            cr.RequestId,
		Account:              account,
		RequestIA:            req.RequestIA,
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
			req.RequestIA, err)
		c.Error500(err, w, r)
		return
	}

	// return join_request_id
	var resp struct {
		RequestId uint64 `json:"request_id"`
	}
	resp.RequestId = connRequest.RequestId
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v new AS: %v, %v",
			account, resp.RequestId, req.RequestIA, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Connection Request Successfully Received: %v Request ID: %v",
		account, resp.RequestId)
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnReply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RespondIA string    `json:"respond_ia"`
		Reply     ConnReply `json:"reply"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, req.RespondIA)
	if err != nil {
		return
	}
	reply := req.Reply
	connReply := models.ConnReply{
		RequestId:   reply.RequestId,
		Account:     account,
		Status:      reply.Status,
		RespondIA:   req.RespondIA,
		RequestIA:   reply.RequestIA,
		Certificate: reply.Certificate, // cert of the replying AS, from the req
		IP:          reply.IP,
		Port:        reply.Port,
		OverlayType: reply.OverlayType,
		MTU:         reply.MTU,
		Bandwidth:   reply.Bandwidth,
	}
	if err := connReply.Insert(); err != nil {
		log.Printf("Error inserting Connection Reply. Request ID: %v Account: %v AS: %v: %v",
			reply.RequestId, account, req.RespondIA, err)
		c.BadRequest(err, w, r)
		return
	}
	// Change the conn req's status to approved.
	cr, err := models.FindConnRequestByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	cr.Status = reply.Status
	if err := cr.Update(); err != nil {
		log.Printf("Error updating conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.RespondIA, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Connection Reply Successfully Received. Account: %v Request ID: %v "+
		"Requesting AS: %v Replying AS: %v", account, reply.RequestId, reply.RequestIA,
		req.RespondIA)
	fmt.Fprintln(w, "{}")
}

func (c *ASController) PollEvents(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs string `json:"isdas"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}
	isdas := req.IsdAs
	account, err := c.findAndValidateAccount(w, r, req.IsdAs)
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
	connRequests, err := models.FindConnRequestsByIsdAs(isdas)
	if err != nil {
		log.Printf("Error while retrieving connection requests. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(err, w, r)
		return
	}
	connReplies, err := models.FindConnRepliesByIsdAs(isdas)
	if err != nil {
		log.Printf("Error while retrieving connection replies. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(err, w, r)
		return
	}

	var resp struct {
		JoinRequests []models.JoinRequest `json:"join_requests"`
		ConnRequests []models.ConnRequest `json:"conn_requests"`
		ConnReplies  []models.ConnReply   `json:"conn_replies"`
	}
	resp.JoinRequests = joinRequests
	resp.ConnRequests = connRequests
	resp.ConnReplies = connReplies

	// log.Printf("join_requests for ISD-AS %v = %v", isdas, joinRequests)
	// log.Printf("conn_requests for ISD-AS %v = %v", isdas, connRequests)
	// log.Printf("conn_replies for ISD-AS %v = %v", isdas, connReplies)

	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling. Account: %v, ISD-AS: %v, %v", account, isdas,
			err)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}
