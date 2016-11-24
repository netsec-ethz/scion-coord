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

type ASController struct {
	controllers.HTTPController
}

type ConnRequest struct {
	Info           string `json:"info"` // free form text motivation for the request
	IsdAsToConnect string `json:"isdas_to_connect"`
	IP             string `json:"ip"`
	Port           uint64 `json:"port"`
	OverlayType    string `json:"overlay_type"`
	MTU            uint64 `json:"mtu"`
	Bandwidth      uint64 `json:"bandwidth"`
	LinkType       string `json:"link_type"`
	Timestamp      string `json:"timestamp"` // UTC ISO 8601 format string
}

type ConnReply struct {
	RequestId      uint64 `json:"request_id" orm:"pk"`
	ReplyingIsdAs  string `json:"replying_isdas"`
	RequesterIsdAs string `json:"requester_isdas"`
	IP             string `json:"ip"`
	Port           uint64 `json:"port"`
	OverlayType    string `json:"overlay_type"`
	MTU            uint64 `json:"mtu"`
	Bandwidth      uint64 `json:"bandwidth"`
}

type JoinRequest struct {
	IsdToJoin uint64 `json:"isd_to_join"`
	SigKey    string `json:"sigkey"`
	EncKey    string `json:"enckey"`
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
		IsdAs:  core_as.IsdAs,
		SigKey: request.SigKey,
		EncKey: request.EncKey,
		Status: models.PENDING,
	}
	// insert into the join_requests table in the database
	if err := join_request.Insert(); err != nil {
		log.Printf("Error inserting join request for core AS %v: %v", core_as, err)
		c.Error500(err, w, r)
		return
	}
	mapping := models.JoinRequestMapping{
		Id:      join_request.Id,
		Account: account,
		IsdAs:   core_as.IsdAs,
	}
	if err := mapping.Insert(); err != nil {
		log.Printf("Error inserting to Join Request Mapping for request "+
			"%v, account: %v, core AS: %v, %v", join_request.Id, account,
			core_as.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	// return join_request_id
	var resp struct {
		Id uint64 `json:"id"`
	}
	resp.Id = join_request.Id
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v core AS: %v ISD %v, %v",
			account, resp.Id, core_as.IsdAs, isd_to_join, err)
		fmt.Fprintln(w, err)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadJoinReply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs     string           `json:"isdas"`
		JoinReply models.JoinReply `json:"join_reply"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		c.BadRequest(err, w, r)
		return
	}
	reply := req.JoinReply
	account, err := c.findAndValidateAccount(w, r, req.IsdAs)
	if err != nil {
		return
	}
	jrm, err := models.FindJoinMappingByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding mapping by join request. Account: %v Request ID: %v, %v",
			account, reply.RequestId, err)
		c.Error500(err, w, r)
		return
	}
	if jrm.IsdAs != req.IsdAs {
		log.Printf("IsdAs %v and Request ID %v do not match.", jrm.IsdAs, reply.RequestId)
		c.BadRequest(fmt.Errorf("IsdAs %v and Request ID %v do not match.",
			jrm.IsdAs, reply.RequestId), w, r)
		return
	}
	if err := reply.Insert(); err != nil {
		log.Printf("Error inserting join reply. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	// Change the join req's status to approved.
	jr, err := models.FindJoinRequestByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	jr.Status = models.APPROVED
	if err := jr.Update(); err != nil {
		log.Printf("Error updating join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}

	log.Printf("JoinReply successfully received. Account: %v Request ID: %v ISD-AS: %v",
		account, reply.RequestId, req.IsdAs)
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
		log.Printf("Error finding account for request: %v: %v",
			request.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	jrm, err := models.FindJoinMappingByRequestId(request.RequestId)
	if err != nil {
		log.Printf("Error finding mapping by join request. Account: %v Request ID: %v, %v",
			account, request.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	if jrm.Account.Id != account.Id {
		log.Printf("Account %v and Request ID %v do not match", account,
			request.RequestId)
		c.BadRequest(fmt.Errorf("Account %v and Request ID %v do not match",
			account, request.RequestId), w, r)
		return
	}
	joinReply, err := models.FindJoinReplyByRequestId(request.RequestId)
	if err != nil {
		log.Printf("Error fetching join reply. Account: %v Request ID: %v, %v",
			account, request.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	isd, err := models.IsdAsToIsd(joinReply.JoiningIsdAs)
	if err != nil {
		log.Printf("Error extracting ISD from ISD-AS %v, %v "+
			joinReply.JoiningIsdAs, err)
		c.Error500(err, w, r)
		return
	}
	new_as := models.As{
		IsdAs:   joinReply.JoiningIsdAs,
		Isd:     isd,
		Core:    false,
		Account: account,
		Created: time.Now().UTC(),
	}
	if new_as.Insert(); err != nil {
		log.Printf("Error inserting new AS: %v Account: %v Request ID: %v, %v",
			new_as.IsdAs, account, request.RequestId, err)
		c.Error500(err, w, r)
		return
	}
	reply := struct {
		IsdAs       string `json:"assigned_isdas"`
		Certificate string `json:"certificate"`
		TRC         string `json:"trc"`
	}{joinReply.JoiningIsdAs, joinReply.Certificate, joinReply.TRC}

	b, err := json.Marshal(reply)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v new AS: %v, %v",
			account, request.RequestId, joinReply.JoiningIsdAs, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("New AS successfully created. Account: %v Request ID: %v new AS: %v",
		account, request.RequestId, joinReply.JoiningIsdAs)
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs       string      `json:"isdas"`
		ConnRequest ConnRequest `json:"request"`
		Signature   string      `json:"signature"` // signature is over IsdAs and ConnRequest
		Certificate string      `json:"certificate"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, req.IsdAs)
	if err != nil {
		return
	}
	cr := req.ConnRequest
	connRequest := models.ConnRequest{
		IsdAsToConnect:       cr.IsdAsToConnect,
		RequesterIsdAs:       req.IsdAs,
		Info:                 cr.Info,
		IP:                   cr.IP,
		Port:                 cr.Port,
		OverlayType:          cr.OverlayType,
		MTU:                  cr.MTU,
		Bandwidth:            cr.Bandwidth,
		LinkType:             cr.LinkType,
		Timestamp:            cr.Timestamp,
		Signature:            req.Signature,
		RequesterCertificate: req.Certificate,
		Status:               models.PENDING,
	}
	if err := connRequest.Insert(); err != nil {
		log.Printf("Error inserting Connection Request. Account %v AS %v: %v", account,
			req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	crm := models.ConnRequestMapping{
		RequestId:      connRequest.Id,
		RequesterIsdAs: req.IsdAs,
		ServerIsdAs:    cr.IsdAsToConnect,
	}
	if err := crm.Insert(); err != nil {
		log.Printf("Error inserting Connection Request Mapping for request "+
			"%v, account: %v, AS: %v, %v", connRequest.Id, account, req.IsdAs, err)
		c.BadRequest(err, w, r)
		return
	}
	var resp struct {
		Id uint64 `json:"id"`
	}
	resp.Id = connRequest.Id
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v new AS: %v, %v",
			account, resp.Id, req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Connection Request Successfully Received: %v Request ID: %v", account, resp.Id)
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnReply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs       string    `json:"isdas"`
		Certificate string    `json:"certificate"`
		Reply       ConnReply `json:"reply"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		c.BadRequest(err, w, r)
		return
	}
	account, err := c.findAndValidateAccount(w, r, req.IsdAs)
	if err != nil {
		return
	}
	reply := req.Reply
	connReply := models.ConnReply{
		RequestId:      reply.RequestId,
		ReplyingIsdAs:  req.IsdAs,
		RequesterIsdAs: reply.RequesterIsdAs,
		Certificate:    req.Certificate, // cert of the replying AS, from the req
		IP:             reply.IP,
		Port:           reply.Port,
		OverlayType:    reply.OverlayType,
		MTU:            reply.MTU,
		Bandwidth:      reply.Bandwidth,
	}

	crm, err := models.FindConnMappingByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding mapping by conn request. Account: %v Request ID: %v, %v",
			account, reply.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	reply.RequesterIsdAs = crm.RequesterIsdAs
	if req.IsdAs != crm.ServerIsdAs {
		log.Printf("IsdAs %v and Request ID %v do not match.", req.IsdAs, crm.RequesterIsdAs)
		c.BadRequest(errors.New("IsdAs and request ID do not match"), w, r)
		return
	}
	if err := connReply.Insert(); err != nil {
		log.Printf("Error inserting Connection Reply. Request ID: %v Account: %v AS: %v: %v",
			reply.RequestId, account, req.IsdAs, err)
		c.BadRequest(err, w, r)
		return
	}
	// Change the conn req's status to approved.
	cr, err := models.FindConnRequestByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	cr.Status = models.APPROVED
	if err := cr.Update(); err != nil {
		log.Printf("Error updating conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, req.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	log.Printf("Connection Reply Successfully Received. Account: %v Request ID: %v AS: %v",
		account, reply.RequestId, req.IsdAs)
	fmt.Fprintln(w, "{}")
}

func (c *ASController) PollEvents(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs string `json:"isdas"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
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

	log.Printf("join_requests for ISD-AS %v = %v", isdas, joinRequests)
	log.Printf("conn_requests for ISD-AS %v = %v", isdas, connRequests)
	log.Printf("conn_replies for ISD-AS %v = %v", isdas, connReplies)

	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling. Account: %v, ISD-AS: %v", account, isdas)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}
