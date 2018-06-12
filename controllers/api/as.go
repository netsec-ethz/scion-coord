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
	"github.com/scionproto/scion/go/lib/addr"
)

// TODO(mlegner): Almost the same struct definitions in `models/events.go`

type JoinRequest struct {
	RequestID     uint64
	Info          string // free form text motivation for the request
	ISDToJoin     int    // the ISD that the sender wants to join
	JoinAsACoreAS bool   // whether to join the ISD as a core AS
	RequesterID   string // the string to identify which account made the request
	SigPubKey     string // signing public key
	EncPubKey     string // encryption public key
}

type JoinReply struct {
	RequestID            uint64
	Status               string
	Info                 string // free form text for the reply
	JoiningIA            string
	IsCore               bool   // whether the new AS joins as core
	RequesterID          string // the string to identify which account made the request
	RespondIA            string
	JoiningIACertificate string `orm:"type(text)"` // certificate of the newly joining AS
	RespondIACertificate string `orm:"type(text)"` // certificate of the responding AS
	TRC                  string `orm:"type(text)"`
}

type ConnRequest struct {
	RequestID   uint64
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
	RequestID   uint64
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
	accountID := mux.Vars(r)["account_id"]

	// get the account from the accountID and secret
	if accountID == "" {
		accountID = r.URL.Query().Get("account_id")
	}

	// find the account belonging to the request
	return models.FindAccountByAccountID(accountID)
}

func ValidateAccountOwnsIA(account *models.Account, ia string) (bool, error) {
	as, err := models.FindASInfoByIA(ia)
	if err != nil {
		return false, err
	}
	return as.Account.ID == account.ID, nil
}

type ASInfoController struct {
	controllers.HTTPController
}

func (c *ASInfoController) Exists(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ia := vars["ia"]

	if ia == "" {
		c.BadRequest(w, nil, "Missing IA parameter")
		return
	}

	if _, err := models.FindASInfoByIA(ia); err != nil {
		c.NotFound(w, nil, ia+" not found")
		return
	}

	fmt.Fprintln(w, "{}") //TODO: why no status ok?
}

// Find the account using the 'account_id' in the request
// and ensure the account owns the concerned ISD-AS
func (c *ASInfoController) findAndValidateAccount(w http.ResponseWriter, r *http.Request,
	ia string) (*models.Account, error) {

	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account. AccountID: %v, Request: %v: %v", mux.Vars(r)["account_id"], r, err)
		c.BadRequest(w, err, "Error finding account")
		return nil, err
	}
	owns, err := ValidateAccountOwnsIA(account, ia)
	if err != nil {
		log.Printf("Error validating account %v owns ISD-AS %v: %v", account, ia, err)
		c.Error500(w, err, "Error validating account %v owns ISD-AS %v", account, ia)
		return nil, err
	}
	if !owns {
		log.Printf("Account %v and AS %v do not match.", account, ia)
		c.Forbidden(w, err, "Account %v and AS %v do not match.", account, ia)
		return nil, err
	}
	return account, nil
}

func (c *ASInfoController) UploadJoinRequest(w http.ResponseWriter, r *http.Request) {
	var request JoinRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	// find the account belonging to the request
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account for request: %v: %v", request, err)
		c.Error500(w, err, "Error finding account for request")
		return
	}
	isdToJoin := request.ISDToJoin
	// find core AS in the ISD to join
	coreASes, err := models.FindCoreASInfosByISD(isdToJoin)
	if err != nil {
		c.BadRequest(w, err, "Error finding core AS in ISD to join")
		return
	}
	if len(coreASes) == 0 {
		log.Printf("ISD %v not found or no core ASes exist for this ISD. Account: %v",
			isdToJoin, account)
		c.Error500(w, err, "ISD not found or no core ASes exist for this ISD")
		return
	}
	// TODO(ercanucan): Send the request to ALL core ASes in this ISD.
	coreAS := coreASes[0]
	joinRequest := models.JoinRequest{
		RequestID:     request.RequestID,
		Info:          request.Info,
		ISDToJoin:     request.ISDToJoin,
		JoinAsACoreAS: request.JoinAsACoreAS,
		RequesterID:   mux.Vars(r)["account_id"],
		RespondIA:     coreAS.String(),
		SigPubKey:     request.SigPubKey,
		EncPubKey:     request.EncPubKey,
		Status:        models.Pending,
	}
	// insert into the join_requests table in the database
	if err := joinRequest.Insert(); err != nil {
		log.Printf("Error inserting join request for core AS %v: %v", coreAS, err)
		c.Error500(w, err, "Error inserting join request")
		return
	}
	log.Printf("Join request successfully received. ISDToJoin: %v Account: %v "+
		"RequesterID: %v", isdToJoin, account, joinRequest.RequesterID)
	fmt.Fprintln(w, "{}")
}

func (c *ASInfoController) UploadJoinReply(w http.ResponseWriter, r *http.Request) {
	var reply JoinReply
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reply); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	account, err := models.FindAccountByAccountID(reply.RequesterID)
	if err != nil {
		log.Printf("Error finding account by AccountID. AccountID: %v, Request ID: %v ISD-AS: %v, %v",
			reply.RequesterID, reply.RequestID, reply.RespondIA, err)
		return
	}
	joinReply := models.JoinReply{
		RequestID:            reply.RequestID,
		RequesterID:          reply.RequesterID,
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
			account, joinReply.RequestID, reply.RespondIA, err)
		c.Error500(w, err, "Error inserting join reply")
		return
	}
	// Change the join request's status to approved/rejected.
	joinRequest, err := models.FindJoinRequest(account.AccountID, joinReply.RequestID)
	if err != nil {
		log.Printf("Error finding join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestID, reply.RespondIA, err)
		c.Error500(w, err, "Error finding join request")
		return
	}
	joinRequest.Status = reply.Status
	if err := joinRequest.Update(); err != nil {
		log.Printf("Error updating join req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, joinReply.RequestID, reply.RespondIA, err)
		c.Error500(w, err, "Error updating join request")
		return
	}
	log.Printf("Received a join reply. Account: %v Request ID: %v ISD-AS: %v Status: %v",
		account, joinReply.RequestID, reply.RespondIA, reply.Status)
	if reply.Status == models.Approved {
		ia, err := addr.IAFromString(joinReply.JoiningIA)
		if err != nil {
			log.Printf("Error parsing ISD-AS %v, %v ", joinReply.JoiningIA, err)
			c.Error500(w, err, "Error parsing ISD-AS")
			return
		}
		newAS := models.ASInfo{
			ISD:     ia.I,
			ASID:    ia.A,
			Core:    joinReply.IsCore,
			Account: account,
			Created: time.Now().UTC(),
		}
		if dbErr := newAS.Insert(); dbErr != nil {
			log.Printf("Error inserting new AS: %v Account: %v Request ID: %v, %v",
				newAS.String(), account, reply.RequestID, err)
			c.Error500(w, dbErr, "Error inserting new AS")
			return
		}
		log.Printf("New AS successfully created. Account: %v Request ID: %v new AS: %v",
			account, reply.RequestID, reply.JoiningIA)
	}
	fmt.Fprintln(w, "{}")
}

func (c *ASInfoController) PollJoinReply(w http.ResponseWriter, r *http.Request) {
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
	joinReply, err := models.FindJoinReply(account.AccountID, request.RequestId)
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
		RequestID:            joinReply.RequestID,
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
		c.Error500(w, err, "Error during JSON marshaling")
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASInfoController) UploadConnRequest(w http.ResponseWriter, r *http.Request) {
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
		RequestID:            cr.RequestID,
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
		Status:               models.Pending,
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
		account, cr.RequestID)
	fmt.Fprintln(w, "{}")
}

func (c *ASInfoController) UploadConnReply(w http.ResponseWriter, r *http.Request) {
	var reply ConnReply
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&reply); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	as, err := models.FindASInfoByIA(reply.RequestIA)
	if err != nil {
		log.Printf("Error finding the RequestIA. Request ID: %v RequestIA: %v, RespondIA: %v, %v",
			reply.RequestID, reply.RequestIA, reply.RespondIA, err)
		c.Error500(w, err, "Error finding the RequestIA")
		return
	}
	account := as.Account
	connReply := models.ConnReply{
		RequestID:   reply.RequestID,
		Account:     account,
		Status:      reply.Status,
		RespondIA:   reply.RespondIA,
		RequestIA:   reply.RequestIA,
		Certificate: reply.Certificate, // Certificate of the replying AS
		IP:          reply.IP,
		Port:        reply.Port,
		OverlayType: reply.OverlayType,
		MTU:         reply.MTU,
		Bandwidth:   reply.Bandwidth,
	}
	if err := connReply.Insert(); err != nil {
		log.Printf("Error inserting Connection Reply. Request ID: %v Account: %v AS: %v: %v",
			reply.RequestID, account, reply.RespondIA, err)
		c.BadRequest(w, err, "Error inserting connection reply")
		return
	}
	// Change the connection request's status to approved/rejected.
	cr, err := models.FindConnRequest(account, reply.RequestID)
	if err != nil {
		log.Printf("Error finding conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestID, reply.RespondIA, err)
		c.Error500(w, err, "Error finding connection request")
		return
	}
	cr.Status = reply.Status
	if err := cr.Update(); err != nil {
		log.Printf("Error updating conn req. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestID, reply.RespondIA, err)
		c.Error500(w, err, "Error updating connection request")
		return
	}

	// Add / subtract the credits for the request depending on the ConnReply.Status
	// Approved = add credits to the receiver
	// Denied = give credits back to the initiator
	if err := c.checkAndUpdateCreditsAtResponse(w, r, cr, reply); err != nil {
		// The function itself takes care of logging eventual errors and writing back the response
		return
	}

	log.Printf("Connection Reply Successfully Received. Account: %v Request ID: %v "+
		"Requesting AS: %v Replying AS: %v Status: %v", account, reply.RequestID, reply.RequestIA,
		reply.RespondIA, reply.Status)
	fmt.Fprintln(w, "{}")
}

// Converts the DB layer JoinRequest array to API layer JoinRequest array
func (c *ASInfoController) prepJoinRequests(in []models.JoinRequest) []JoinRequest {
	if in == nil || len(in) == 0 {
		return make([]JoinRequest, 0)
	}
	out := make([]JoinRequest, len(in))
	for i, v := range in {
		out[i] = JoinRequest{
			RequestID:     v.RequestID,
			Info:          v.Info,
			ISDToJoin:     v.ISDToJoin,
			JoinAsACoreAS: v.JoinAsACoreAS,
			SigPubKey:     v.SigPubKey,
			EncPubKey:     v.EncPubKey,
			RequesterID:   v.RequesterID,
		}
	}
	return out
}

// Converts the DB layer ConnRequest array to API layer ConnRequest array
func (c *ASInfoController) prepConnRequests(in []models.ConnRequest) []ConnRequest {
	if in == nil || len(in) == 0 {
		return make([]ConnRequest, 0)
	}
	out := make([]ConnRequest, len(in))
	for i, v := range in {
		out[i] = ConnRequest{
			RequestID:   v.RequestID,
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
func (c *ASInfoController) prepConnReplies(in []models.ConnReply) []ConnReply {
	if in == nil || len(in) == 0 {
		return make([]ConnReply, 0)
	}
	out := make([]ConnReply, len(in))
	for i, v := range in {
		out[i] = ConnReply{
			RequestID:   v.RequestID,
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
func (c *ASInfoController) PollEvents(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IA string
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}

	ia := req.IA
	account, err := c.findAndValidateAccount(w, r, ia)
	if err != nil {
		// findAndValidateAccount logs the error and writes back the response
		return
	}
	joinRequests, err := models.FindOpenJoinRequestsByIA(ia)
	if err != nil {
		log.Printf("Error while retrieving open join requests. Account: %v, ISD-AS: %v",
			account, ia)
		c.BadRequest(w, err, "Error while retrieving open join requests")
		return
	}
	connRequests, err := models.FindOpenConnRequestsByRespondIA(ia)
	if err != nil {
		log.Printf("Error while retrieving connection requests. Account: %v, ISD-AS: %v",
			account, ia)
		c.BadRequest(w, err, "Error while retrieving connection Requests")
		return
	}
	connReplies, err := models.FindConnRepliesByRequestIA(ia)
	if err != nil {
		log.Printf("Error while retrieving connection replies. Account: %v, ISD-AS: %v",
			account, ia)
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
		log.Printf("Error during JSON marshaling. Account: %v, ISD-AS: %v, %v", account, ia,
			err)
		c.Error500(w, err, "Error during JSON marshaling")
		return
	}
	fmt.Fprintln(w, string(b))
}

// Converts the list of DB AS objects into a list of strings.
func (c *ASInfoController) prepASListResponse(in []models.ASInfo) []string {
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
func (c *ASInfoController) ListASes(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IA string
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error decoding JSON")
		return
	}
	account, err := c.findAndValidateAccount(w, r, req.IA)
	if err != nil {
		// findAndValidateAccount logs the error and writes back the response
		return
	}
	ia, err := addr.IAFromString(req.IA)
	if err != nil {
		log.Printf("Error parsing ISD-AS %v, %v ", req.IA, err)
		c.Error500(w, err, "Error parsing ISD-AS")
		return
	}
	ases, err := models.FindASInfosByISD(ia.I)
	if err != nil {
		log.Printf("Error while retrieving list of ASes. Account: %v, ISD-AS: %v", account,
			req.IA)
		c.BadRequest(w, err, "Error while retrieving list of ASes")
		return
	}
	resp := c.prepASListResponse(ases)
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON marshaling. Account: %v, ISD-AS: %v, %v", account, req.IA,
			err)
		c.Error500(w, err, "Error during JSON marshaling")
		return
	}
	fmt.Fprintln(w, string(b))
}
