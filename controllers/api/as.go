package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/models"
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
		Core  bool `json:"core"`
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
		AsToQuery string `json:"as_to_query"`
		SigKey    string `json:"sigkey"`
		EncKey    string `json:"enckey"`
	}

	var request JoinRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		c.BadRequest(err, w, r)
		return
	}
	// find core AS in the ISD to join
	core_as, err := models.FindCoreAsByIsdAs(request.IsdToJoin, request.AsToQuery)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}

	join_request := models.JoinRequest{
		IsdAs:  core_as.IsdAs,
		AsToQuery: request.AsToQuery,
		SigKey: request.SigKey,
		EncKey: request.EncKey,
	}

	// insert into database
	if err := join_request.Insert(); err != nil {
		c.Error500(err, w, r)
		return
	}
	// find the account belonging to the request
	account, err := models.FindAccountByKey(mux.Vars(r)["key"])
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	mapping := models.JoinRequestMapping{
		Id:      join_request.Id,
		Account: account,
		IsdAs:   core_as.IsdAs,
	}
	if err := mapping.Insert(); err != nil {
		c.Error500(err, w, r)
		return
	}

	// return join_request_id
	var reply struct {
		Id uint64 `json:"id"`
	}
	reply.Id = join_request.Id
	b, err := json.Marshal(reply)
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadJoinReplies(w http.ResponseWriter, r *http.Request) {

	var request struct {
		IsdAs       string             `json:"isdas"`
		JoinReplies []models.JoinReply `json:"replies"`
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
	for _, reply := range request.JoinReplies {
		jrm, err := models.FindJoinMappingByRequestId(reply.RequestId)
		if err != nil {
			c.Error500(err, w, r)
			return
		}
		if jrm.IsdAs != request.IsdAs {
			c.BadRequest(errors.New("IsdAs and Request Id do not match"), w, r)
			return
		}
		isd, err := models.IsdAsToIsd(request.IsdAs)
		if err != nil {
			c.BadRequest(err, w, r)
			return
		}

		reply.IsdAs = fmt.Sprintf("%v-%v", isd, reply.RequestId)

		if err := reply.Insert(); err != nil {
			c.Error500(err, w, r)
			return
		}
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
		c.BadRequest(err, w, r)
		return
	}
	jrm, err := models.FindJoinMappingByRequestId(request.RequestId)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	if jrm.Account.Id != account.Id {
		c.BadRequest(errors.New("Account and request ID do not match"), w, r)
		return
	}

	joinReply, err := models.FindJoinReplyByRequestId(request.RequestId)
	if err != nil {
		c.BadRequest(errors.New("No reply"), w, r)
		return
	}

	isd, err := models.IsdAsToIsd(joinReply.IsdAs)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	finalAs := models.As{
		IsdAs:   joinReply.IsdAs,
		Isd:     isd,
		Core:    false,
		Account: account,
		Created: time.Now().UTC(),
	}

	if err = finalAs.Insert(); err != nil {
		c.Error500(err, w, r)
		return
	}

	reply := struct {
		IsdAs       string `json:"isdas"`
		Certificate string `json:"certificate"`
		TRC         string `json:"trc"`
	}{joinReply.IsdAs, joinReply.Certificate, joinReply.TRC}

	b, err := json.Marshal(reply)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) UploadConnRequests(w http.ResponseWriter, r *http.Request) {

	type ConnRequestInfo struct {
		IsdAs     string `json:"isdas"`
		IP        string `json:"ip"`
		Port      uint64 `json:"port"`
		MTU       uint64 `json:"mtu"`
		Bandwidth uint64 `json:"bandwidth"`
		Linktype  string `json:"linktype"`
	}

	var request struct {
		IsdAs            string            `json:"isdas"`
		Certificate      string            `json:"certificate"`
		ConnRequestInfos []ConnRequestInfo `json:"requests"`
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

	for _, cri := range request.ConnRequestInfos {
		connRequest := models.ConnRequest{
			IsdAs:                cri.IsdAs,
			RequesterIsdAs:       request.IsdAs,
			RequesterCertificate: request.Certificate,
			IP:                   cri.IP,
			Port:                 cri.Port,
			MTU:                  cri.MTU,
			Bandwidth:            cri.Bandwidth,
			Linktype:             cri.Linktype,
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
	var request struct {
		IsdAs       string             `json:"isdas"`
		Certificate string             `json:"certificate"`
		Replies     []models.ConnReply `json:"replies"`
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
		reply.Certificate = request.Certificate

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
		if err := reply.Insert(); err != nil {
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

	joinRequests, err := models.FindJoinRequestsByIsdAs(request.IsdAs)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}

	connRequests, err := models.FindConnRequestsByIsdAs(request.IsdAs)
	if err != nil {
		c.BadRequest(err, w, r)
		return
	}
	var reply struct {
		JoinRequests []models.JoinRequest `json:"join_requests"`
		ConnRequests []models.ConnRequest `json:"conn_requests"`
	}
	reply.JoinRequests = joinRequests
	reply.ConnRequests = connRequests

	b, err := json.Marshal(reply)
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
	return
}
