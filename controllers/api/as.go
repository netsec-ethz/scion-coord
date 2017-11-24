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
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion/go/lib/addr"
)

type JoinRequest struct {
	RequestId     uint64
	Info          string // free form text motivation for the request
	IsdToJoin     int    // the ISD that the sender wants to join
	JoinAsACoreAS bool   // whether to join the ISD as a core AS
	RequesterId   string // the string to identify which account made the request
	SigPubKey     string // signing public key
	EncPubKey     string // encryption public key
}

type JoinReply struct {
	RequestId            uint64
	Status               string
	Info                 string // free form text for the reply
	JoiningIA            string
	IsCore               bool   // whether the new AS joins as core
	RequesterId          string // the string to identify which account made the request
	RespondIA            string
	JoiningIACertificate string `orm:"type(text)"` // certificate of the newly joining AS
	RespondIACertificate string `orm:"type(text)"` // certificate of the responding AS
	TRC                  string `orm:"type(text)"`
}

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

func FindAccountByRequest(r *http.Request) (*models.Account, error) {
	account_id := mux.Vars(r)["account_id"]

	// get the account from the account_id and secret
	if account_id == "" {
		account_id = r.URL.Query().Get("account_id")
	}

	// find the account belonging to the request
	return models.FindAccountByAccountId(account_id)
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
		c.BadRequest(w, nil, "Missing isdas parameter")
		return
	}

	if _, err := models.FindAsByIsdAs(isdas); err != nil {
		c.NotFound(w, nil, isdas+" not found")
		return
	}

	fmt.Fprintln(w, "{}") //TODO: why no status ok?
}

// Find the account using the 'account_id' in the request
// and ensure the account owns the concerned ISD-AS
func (c *ASController) findAndValidateAccount(w http.ResponseWriter, r *http.Request,
	isdas string) (*models.Account, error) {

	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account. AccountID: %v, Request: %v: %v", mux.Vars(r)["account_id"], r, err)
		c.BadRequest(w, err, "Error finding account")
		return nil, err
	}
	owns, err := ValidateAccountOwnsIsdAs(account, isdas)
	if err != nil {
		log.Printf("Error validating account %v owns ISD-AS %v: %v", account, isdas, err)
		c.Error500(w, err, "Error validating account %v owns ISD-AS %v", account, isdas)
		return nil, err
	}
	if !owns {
		log.Printf("Account %v and AS %v do not match.", account, isdas)
		c.Forbidden(w, err, "Account %v and AS %v do not match.", account, isdas)
		return nil, err
	}
	return account, nil
}

func (c *ASController) UploadJoinRequest(w http.ResponseWriter, r *http.Request) {
	var request JoinRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding Json")
		return
	}
	// find the account belonging to the request
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account for request: %v: %v", request, err)
		c.Error500(w, err, "Error finding account for request")
		return
	}
	isd_to_join := request.IsdToJoin
	// find core AS in the ISD to join
	core_ases, err := models.FindCoreASesByIsd(isd_to_join)
	if err != nil {
		c.BadRequest(w, err, "Error finding core AS in ISD to join")
		return
	}
	if len(core_ases) == 0 {
		log.Printf("ISD %v not found or no core ASes exist for this ISD. Account: %v",
			isd_to_join, account)
		c.Error500(w, err, "ISD not found or no core ASes exist for this ISD")
		return
	}
	// TODO(ercanucan): Send the request to ALL core ASes in this ISD.
	core_as := core_ases[0]
	join_request := models.JoinRequest{
		RequestId:     request.RequestId,
		Info:          request.Info,
		IsdToJoin:     request.IsdToJoin,
		JoinAsACoreAS: request.JoinAsACoreAS,
		RequesterId:   mux.Vars(r)["account_id"],
		RespondIA:     core_as.String(),
		SigPubKey:     request.SigPubKey,
		EncPubKey:     request.EncPubKey,
		Status:        models.PENDING,
	}
	// insert into the join_requests table in the database
	if err := join_request.Insert(); err != nil {
		log.Printf("Error inserting join request for core AS %v: %v", core_as, err)
		c.Error500(w, err, "Error inserting join request")
		return
	}
	log.Printf("Join request successfully received. ISDToJoin: %v Account: %v "+
		"RequesterId: %v", isd_to_join, account, join_request.RequesterId)
	fmt.Fprintln(w, "{}")
}

func (c *ASController) UploadJoinReply(w http.ResponseWriter, r *http.Request) {
	var reply JoinReply
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reply); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	account, err := models.FindAccountByAccountId(reply.RequesterId)
	if err != nil {
		log.Printf("Error finding account by AccountID. AccountID: %v, Request ID: %v ISD-AS: %v, %v",
			reply.RequesterId, reply.RequestId, reply.RespondIA, err)
		return
	}
	joinReply := models.JoinReply{
		RequestId:            reply.RequestId,
		RequesterId:          reply.RequesterId,
		Status:               reply.Status,
		JoiningIA:            reply.JoiningIA,
		IsCore:               reply.IsCore,
		RespondIA:            reply.RespondIA,
		JoiningIACertificate: reply.JoiningIACertificate,
		RespondIACertificate: reply.RespondIACertificate,
		TRC:                  reply.TRC,
	}
	if err := joinReply.Insert(); err != nil {
		log.Printf("Error inserting join reply. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, reply.RespondIA, err)
		c.Error500(w, err, "Error inserting join reply")
		return
	}
	// Change the join req's status to approved/rejected.
	join_req, err := models.FindJoinRequest(account.AccountId, joinReply.RequestId)
	if err != nil {
		log.Printf("Error finding join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, reply.RespondIA, err)
		c.Error500(w, err, "Error finding join request")
		return
	}
	join_req.Status = reply.Status
	if err := join_req.Update(); err != nil {
		log.Printf("Error updating join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestId, reply.RespondIA, err)
		c.Error500(w, err, "Error updating join request")
		return
	}
	log.Printf("Received a join reply. Account: %v Request ID: %v ISD-AS: %v Status: %v",
		account, joinReply.RequestId, reply.RespondIA, reply.Status)
	if reply.Status == models.APPROVED {
		ia, err := addr.IAFromString(joinReply.JoiningIA)
		if err != nil {
			log.Printf("Error parsing ISD-AS %v, %v ", joinReply.JoiningIA, err)
			c.Error500(w, err, "Error parsing ISD-AS")
			return
		}
		new_as := models.As{
			Isd:     ia.I,
			As:      ia.A,
			Core:    joinReply.IsCore,
			Account: account,
			Created: time.Now().UTC(),
		}
		if dbErr := new_as.Insert(); dbErr != nil {
			log.Printf("Error inserting new AS: %v Account: %v Request ID: %v, %v",
				new_as.String(), account, reply.RequestId, err)
			c.Error500(w, dbErr, "Error inserting new AS")
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
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account for request: %v: %v", request.RequestId, err)
		c.BadRequest(w, err, "Error finding account for request")
		return
	}
	joinReply, err := models.FindJoinReply(account.AccountId, request.RequestId)
	if err == orm.ErrNoRows {
		log.Printf("No join reply for Account: %v Request ID: %v", account,
			request.RequestId)
		fmt.Fprintln(w, "{}")
		return
	} else if err != nil {
		log.Printf("Error during join reply lookup. Account: %v Request ID: %v, %v",
			account, request.RequestId, err)
		c.Error500(w, err, "Error during join reply lookup")
		return
	}
	reply := JoinReply{
		RequestId:            joinReply.RequestId,
		Status:               joinReply.Status,
		RespondIA:            joinReply.RespondIA,
		IsCore:               joinReply.IsCore,
		JoiningIA:            joinReply.JoiningIA,
		JoiningIACertificate: joinReply.JoiningIACertificate,
		RespondIACertificate: joinReply.RespondIACertificate,
		TRC:                  joinReply.TRC,
	}
	b, err := json.Marshal(reply)
	if err != nil {
		log.Printf("Error marshaling JSON for account: %v request: %v new AS: %v, %v",
			account, request.RequestId, joinReply.JoiningIA, err)
		c.Error500(w, err, "Error marshaling JSON")
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnRequest(w http.ResponseWriter, r *http.Request) {
	var cr ConnRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&cr); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	account, err := c.findAndValidateAccount(w, r, cr.RequestIA)
	if err != nil {
		// findAndValidateAccount logs the error and writes back the response
		return
	}

	// Check if the AS has enough credits for that connection and
	// update new credits for both ASes
	if err := c.checkAndUpdateCredits(w, r, &cr); err != nil {
		// The function itself takes care of logging eventual errors and writing back the response
		return
	}

	connRequest := models.ConnRequest{
		RequestId:            cr.RequestId,
		AccountId:            account,
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
		log.Printf("Error inserting connection request. Account %v AS %v: %v", account,
			cr.RequestIA, err)
		c.Error500(w, err, "Error inserting connection request")
		// The credits will be granted in foresight and must be removed in case of an error (now)
		c.rollBackCreditUpdate(w, r, &cr)
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
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	as, err := models.FindAsByIsdAs(reply.RequestIA)
	if err != nil {
		log.Printf("Error finding the RequestIA. Request ID: %v RequestIA: %v, RespondIA: %v, %v",
			reply.RequestId, reply.RequestIA, reply.RespondIA, err)
		c.Error500(w, err, "Error finding the RequestIA")
		return
	}
	account := as.Account
	connReply := models.ConnReply{
		RequestId:   reply.RequestId,
		AccountId:   account,
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
		log.Printf("Error inserting connection Reply. request ID: %v Account: %v AS: %v: %v",
			reply.RequestId, account, reply.RespondIA, err)
		c.BadRequest(w, err, "Error inserting connection reply")
		return
	}
	// Change the conn req's status to approved/rejected.
	cr, err := models.FindConnRequest(account, reply.RequestId)
	if err != nil {
		log.Printf("Error finding conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, reply.RespondIA, err)
		c.Error500(w, err, "Error finding connection request")
		return
	}
	cr.Status = reply.Status
	if err := cr.Update(); err != nil {
		log.Printf("Error updating conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, reply.RespondIA, err)
		c.Error500(w, err, "Error updating connection request")
		return
	}

	// Add / subtract the credits for the request depending on the ConnReply.Status
	// APPROVED = add credits to the receiver
	// DENIED = give credits back to the initiator
	if err := c.checkAndUpdateCreditsAtResponse(w, r, cr, reply); err != nil {
		// The function itself takes care of logging eventual errors and writing back the response
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
			RequestId:     v.RequestId,
			Info:          v.Info,
			IsdToJoin:     v.IsdToJoin,
			JoinAsACoreAS: v.JoinAsACoreAS,
			SigPubKey:     v.SigPubKey,
			EncPubKey:     v.EncPubKey,
			RequesterId:   v.RequesterId,
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
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}

	isdas := req.IsdAs
	account, err := c.findAndValidateAccount(w, r, isdas)
	if err != nil {
		// findAndValidateAccount logs the error and writes back the response
		return
	}
	joinRequests, err := models.FindOpenJoinRequestsByIsdAs(isdas)
	if err != nil {
		log.Printf("Error while retrieving open join requests. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(w, err, "Error while retrieving open join requests")
		return
	}
	connRequests, err := models.FindOpenConnRequestsByRespondIA(isdas)
	if err != nil {
		log.Printf("Error while retrieving connection requests. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(w, err, "Error while retrieving connection Requests")
		return
	}
	connReplies, err := models.FindConnRepliesByRequestIA(isdas)
	if err != nil {
		log.Printf("Error while retrieving connection replies. Account: %v, ISD-AS: %v",
			account, isdas)
		c.BadRequest(w, err, "Error while retrieving connection replies")
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
		c.Error500(w, err, "Error during JSON Marshaling")
		return
	}
	fmt.Fprintln(w, string(b))
}

// Converts the list of DB AS objects into a list of strings.
func (c *ASController) prepASListResponse(in []models.As) []string {
	var out []string
	if len(in) == 0 {
		return out
	}
	for _, v := range in {
		out = append(out, v.String())
	}
	return out
}

// API end-point to serve the list of ASes available for an AS to connect to.
// Responds back with a list of ASes in the ISD that the AS belongs to.
func (c *ASController) ListASes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IsdAs string
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	account, err := c.findAndValidateAccount(w, r, req.IsdAs)
	if err != nil {
		// findAndValidateAccount logs the error and writes back the response
		return
	}
	ia, err := addr.IAFromString(req.IsdAs)
	if err != nil {
		log.Printf("Error parsing ISD-AS %v, %v ", req.IsdAs, err)
		c.Error500(w, err, "Error parsing ISD-AS")
		return
	}
	ases, err := models.FindASesByIsd(ia.I)
	if err != nil {
		log.Printf("Error while retrieving list of ASes. Account: %v, ISD-AS: %v", account,
			req.IsdAs)
		c.BadRequest(w, err, "Error while retreiving list of ASes")
		return
	}
	resp := c.prepASListResponse(ases)
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling. Account: %v, ISD-AS: %v, %v", account, req.IsdAs,
			err)
		c.Error500(w, err, "Error during JSON Marshaling")
		return
	}
	fmt.Fprintln(w, string(b))
}
