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
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/models"
)

func (c *ASController) ListAsesConnectionsWithCredits(w http.ResponseWriter, r *http.Request) {
	var response struct {
		ISD         int
		AS          int
		Credits     int64
		Connections []models.ConnectionWithCredits
	}

	vars := mux.Vars(r)
	isdas := vars["isdas"]

	if isdas == "" {
		c.BadRequest(errors.New("missing isdas parameter"), w, r)
		return
	}

	requestingAS, err := models.FindAsByIsdAs(isdas)
	if err != nil {
		c.NotFound(errors.New(isdas+" not found"), w, r)
		return
	}
	response.ISD = requestingAS.Isd
	response.AS = requestingAS.As
	response.Credits = requestingAS.Credits

	connections, err := requestingAS.ListConnections()
	if err != nil {
		log.Printf("Error while retrieving list of ASes. ISD-AS: %v", requestingAS)
		c.BadRequest(err, w, r)
		return
	}
	response.Connections = connections

	b, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error during JSON Marshaling. ISD-AS: %v, %v", requestingAS, err)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *ASController) CheckAndUpdateCredits(w http.ResponseWriter, r *http.Request, cr *ConnRequest) error {
	as, err := models.FindAsByIsdAs(cr.RequestIA)
	if err != nil {
		log.Printf("Error: Unkown AS: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return err
	}

	var creditsNeeded = models.BandwidthToCredits(cr.Bandwidth)
	if (as.Credits - creditsNeeded) <= 0 {
		err = errors.New(fmt.Sprintf("Error: Not enough credits to create a connection request! You need %v, but have only %v", creditsNeeded, as.Credits))
		log.Printf("Info: Not enough credits! AS: %v, Request: %v, Error: %v", as, r.Body, err)
		c.BadRequest(err, w, r)
		return err
	}

	// Subtract credits from AS
	if err := as.UpdateCurrency(-1 * creditsNeeded); err != nil {
		log.Printf("Error: Substracting credits! AS: %v, Request: %v, Error: %v", as, r.Body, err)
		c.Error500(err, w, r)
		return err
	}
	return nil
}

func (c *ASController) RollBackCreditUpdate(w http.ResponseWriter, r *http.Request, cr *ConnRequest) {
	as, err := models.FindAsByIsdAs(cr.RequestIA)
	if err != nil {
		log.Printf("Error: Unkown AS: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}

	var creditsNeeded = models.BandwidthToCredits(cr.Bandwidth)
	// Roll back UpdateCurrency changes
	if err := as.UpdateCurrency(creditsNeeded); err != nil {
		log.Printf("Critical error: Can't roll back UpdateCurrency changes! Credits: %v, AS: %v, Request: %v, Error: %v", creditsNeeded, as, r.Body, err)
	}
}
