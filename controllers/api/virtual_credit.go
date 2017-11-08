// Copyright 2017 ETH Zurich
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
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/models"
)

// Dummy error to return if the virtual credit system is disabled
var systemDisabledError error = errors.New("VirtualCredit system disabled. This error should be ignored")

// REST resource to list the payed connections from one AS (identified by the parameter 'isdas')
// Example Response:
/*
{
  "ISD": 1,
  "AS": 1,
  "Credits": 10,
  "Connections": [
    {
      "ISD": 1,
      "AS": 2,
      "CreditBalance": 10,
      "Bandwidth": 100000,
      "IsOutgoing": false,
      "Timestamp": "2017-07-17 13:59:37.396791+00:00"
    }
  ]
}
*/
func (c *ASController) ListAsesConnectionsWithCredits(w http.ResponseWriter, r *http.Request) {
	if config.VIRTUAL_CREDIT_ENABLE == false {
		c.NotFound(systemDisabledError, w, r)
		return
	}

	var response struct {
		ISD         int                            // The ISD of this AS
		AS          int                            // This AS
		Credits     int64                          // The current credits of this AS
		Connections []models.ConnectionWithCredits // List of connections and their costs / yields
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

func (c *ASController) checkAndUpdateCredits(w http.ResponseWriter, r *http.Request, cr *ConnRequest) error {
	if config.VIRTUAL_CREDIT_ENABLE == false {
		return systemDisabledError
	}

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

func (c *ASController) checkAndUpdateCreditsAtResponse(w http.ResponseWriter, r *http.Request, cr *models.ConnRequest, reply ConnReply) error {
	if !config.VIRTUAL_CREDIT_ENABLE == false {
		return systemDisabledError
	}

	as, _ := models.FindAsByIsdAs(cr.RequestIA)
	var credits = models.BandwidthToCredits(cr.Bandwidth)
	// If the connection is approved, add credits to the responding AS
	if cr.Status == models.APPROVED {
		otherAs, err := models.FindAsByIsdAs(cr.RespondIA)
		if err != nil {
			log.Printf("Error finding the RespondIA. Request ID: %v RequestIA: %v, RespondIA: %v, %v",
				reply.RequestId, reply.RequestIA, reply.RespondIA, err)
			c.Error500(err, w, r)
			return err
		}
		if err := otherAs.UpdateCurrency(credits); err != nil {
			log.Printf("Error: Adding credits! OtherAS: %v, Request: %v, Error: %v", otherAs, r.Body, err)
			c.Error500(err, w, r)
			return err
		}
		// If the connection request is denied, release the reserved credits for the connection
	} else if cr.Status != models.PENDING {
		if err := as.UpdateCurrency(credits); err != nil {
			log.Printf("Error: Readding credits! ThisAS: %v, Request: %v, Error: %v", as, r.Body, err)
			c.Error500(err, w, r)
			return err
		}
	}
	return nil
}

func (c *ASController) rollBackCreditUpdate(w http.ResponseWriter, r *http.Request, cr *ConnRequest) {
	if !config.VIRTUAL_CREDIT_ENABLE {
		return
	}

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
