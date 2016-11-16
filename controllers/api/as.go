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

func (c *ASController) UploadJoinRequest(w http.ResponseWriter, r *http.Request) {

	type JoinRequest struct {
		IsdToJoin uint64 `json:"isd_to_join"`
		SigKey    string `json:"sigkey"`
		EncKey    string `json:"enckey"`
	}
	var request JoinRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}
	// find the account belonging to the request
	key := mux.Vars(r)["key"]
	account, err := models.FindAccountByKey(key)
	if err != nil {
		log.Printf("Error finding account for key %v: %v", key, err)
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

	var request struct {
		IsdAs     string           `json:"isdas"`
		JoinReply models.JoinReply `json:"join_reply"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Printf("Error decoding JSON: %v", err)
		c.BadRequest(err, w, r)
		return
	}
	reply := request.JoinReply
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error finding account for request: %v: %v", reply.RequestId, err)
		c.BadRequest(err, w, r)
		return
	}
	owns, err := ValidateAccountOwnsIsdAs(account, request.IsdAs)
	if err != nil {
		log.Printf("Error validating account %v owns ISD-AS %v: %v", account, request.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	if !owns {
		log.Printf("Account %v and AS %v do not match.", account, request.IsdAs)
		c.BadRequest(fmt.Errorf("Account %v and AS %v do not match.", account,
			request.IsdAs), w, r)
		return
	}
	jrm, err := models.FindJoinMappingByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding mapping by join request. Account: %v Request ID: %v, %v",
			account, reply.RequestId, err)
		c.Error500(err, w, r)
		return
	}
	if jrm.IsdAs != request.IsdAs {
		log.Printf("IsdAs %v and Request ID %v do not match.", jrm.IsdAs, reply.RequestId)
		c.BadRequest(fmt.Errorf("IsdAs %v and Request ID %v do not match.",
			jrm.IsdAs, reply.RequestId), w, r)
		return
	}
	if err := reply.Insert(); err != nil {
		log.Printf("Error inserting join reply. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, request.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	// Change the join request's status to approved.
	jr, err := models.FindJoinRequestByRequestId(reply.RequestId)
	if err != nil {
		log.Printf("Error finding join request. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, request.IsdAs, err)
		c.Error500(err, w, r)
		return
	}
	jr.Status = models.APPROVED
	if err := jr.Update(); err != nil {
		log.Printf("Error updating join request. Account: %v Request ID: %v ISD-AS: %v, %v",
			account, reply.RequestId, request.IsdAs, err)
		c.Error500(err, w, r)
		return
	}

	log.Printf("JoinReply successfully received. Account: %v Request ID: %v ISD-AS: %v",
		account, reply.RequestId, request.IsdAs)
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

	type ConnRequestInfo struct {
		Info      string `json:"info"` // free form text motivation for the request
		IsdAs     string `json:"isdas"`
		IP        string `json:"ip"`
		Port      uint64 `json:"port"`
		MTU       uint64 `json:"mtu"`
		Bandwidth uint64 `json:"bandwidth"`
		Linktype  string `json:"linktype"`
		Timestamp string `json:"timestamp"` // UTC ISO 8601 format string
	}

	var request struct {
		IsdAs            string          `json:"isdas"`
		ConnRequestInfos ConnRequestInfo `json:"request"`
		Signature        string          `json:"signature"` // signature is over IsdAs and ConnRequestInfos
		Certificate      string          `json:"certificate"`
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}

	account, err := FindAccountByRequest(r)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	owns, err := ValidateAccountOwnsIsdAs(account, request.IsdAs)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	if !owns {
		c.BadRequest(errors.New("Account and AS do not match"), w, r)
		return
	}

	var ids []uint64

	cri := request.ConnRequestInfos

	connRequest := models.ConnRequest{
		IsdAs:                cri.IsdAs,
		RequesterIsdAs:       request.IsdAs,
		Info:                 cri.Info,
		IP:                   cri.IP,
		Port:                 cri.Port,
		MTU:                  cri.MTU,
		Bandwidth:            cri.Bandwidth,
		Linktype:             cri.Linktype,
		Signature:            request.Signature,
		RequesterCertificate: request.Certificate,
	}

	// upsert it
	if err := connRequest.Insert(); err != nil {
		c.Error500(err, w, r)
		return
	}
	ids = append(ids, connRequest.Id)
	crm := &models.ConnRequestMapping{
		RequestId:      connRequest.Id,
		RequesterIsdAs: request.IsdAs,
		ServerIsdAs:    cri.IsdAs,
	}
	if err := crm.Insert(); err != nil {
		c.BadRequest(err, w, r)
		return
	}

	// return join_request_id
	var reply struct {
		Ids []uint64 `json:"ids"`
	}
	reply.Ids = ids
	b, err := json.Marshal(reply)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnReplies(w http.ResponseWriter, r *http.Request) {
	type ConnReplyInfo struct {
		RequestId      uint64 `json:"request_id" orm:"pk"`
		RequesterIsdAs string `json:"requester_isdas"`
		IP             string `json:"ip"`
		Port           uint64 `json:"port"`
		MTU            uint64 `json:"mtu"`
		Bandwidth      uint64 `json:"bandwidth"`
	}
	var request struct {
		IsdAs       string          `json:"isdas"`
		Certificate string          `json:"certificate"`
		Replies     []ConnReplyInfo `json:"replies"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}
	account, err := FindAccountByRequest(r)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	owns, err := ValidateAccountOwnsIsdAs(account, request.IsdAs)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	if !owns {
		c.BadRequest(errors.New("Account and AS do not match"), w, r)
		return
	}
	for _, reply := range request.Replies {

		connReply := models.ConnReply{
			RequestId:      reply.RequestId,
			RequesterIsdAs: reply.RequesterIsdAs,
			Certificate:    request.Certificate, // cert of the replying AS, from the request
			IP:             reply.IP,
			Port:           reply.Port,
			MTU:            reply.MTU,
			Bandwidth:      reply.Bandwidth,
		}

		crm, err := models.FindConnMappingByRequestId(reply.RequestId)
		if err != nil {
			c.BadRequest(err, w, r)
			return
		}
		reply.RequesterIsdAs = crm.RequesterIsdAs
		if request.IsdAs != crm.ServerIsdAs {
			c.BadRequest(errors.New("IsdAs and request ID do not match"), w, r)
			return
		}
		if err := connReply.Insert(); err != nil {
			c.BadRequest(err, w, r)
			return
		}
	}
	fmt.Fprintln(w, "{}")
}

func (c *ASController) PollConnReplies(w http.ResponseWriter, r *http.Request) {

	var request struct {
		IsdAs string `json:"isdas"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}
	account, err := FindAccountByRequest(r)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	owns, err := ValidateAccountOwnsIsdAs(account, request.IsdAs)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	if !owns {
		c.BadRequest(errors.New("Account and AS do not match"), w, r)
		return
	}
	replies, err := models.FindConnRepliesByIsdAs(request.IsdAs)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	if len(replies) == 0 {
		fmt.Fprint(w, "{}")
		return
	}

	var reply struct {
		Replies []models.ConnReply `json:"replies"`
	}
	reply.Replies = replies
	b, err := json.Marshal(reply)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) PollEvents(w http.ResponseWriter, r *http.Request) {
	var request struct {
		IsdAs string `json:"isdas"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}
	account, err := FindAccountByRequest(r)
	if err != nil {
		log.Printf("Error while retrieving account for request %v", r)
		c.BadRequest(err, w, r)
		return
	}
	isdas := request.IsdAs
	owns, err := ValidateAccountOwnsIsdAs(account, isdas)
	if err != nil {
		log.Printf("Error validating acc %v for isdas %v", account, isdas)
		c.Error500(err, w, r)
		return
	}
	if !owns {
		log.Printf("Account and AS do not match %v: %v", account, isdas)
		c.BadRequest(errors.New("Account and AS do not match"), w, r)
		return
	}

	joinRequests, err := models.FindOpenJoinRequestsByIsdAs(isdas)
	if err != nil {
		log.Printf("Error while retrieving open join requests for %v", isdas)
		c.BadRequest(err, w, r)
		return
	}

	connRequests, err := models.FindConnRequestsByIsdAs(isdas)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}

	var resp struct {
		JoinRequests []models.JoinRequest `json:"join_requests"`
		ConnRequests []models.ConnRequest `json:"conn_requests"`
	}
	resp.JoinRequests = joinRequests
	resp.ConnRequests = connRequests

	log.Printf("join_requests = %v", joinRequests)
	log.Printf("conn_requests = %v", connRequests)

	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling")
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}
