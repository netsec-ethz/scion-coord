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
	"github.com/scionproto/scion/go/lib/addr"
)

// Dummy error to return if the virtual credit system is disabled
var systemDisabledError = errors.New(
	"VirtualCredit system disabled. This error should be ignored")

// REST resource to list the payed connections from one AS (identified by the parameter 'ia')
// Example Response:
//
//{
//  "ISD": 1,
//  "AS": 1,
//  "Credits": 10,
//  "Connections": [
//      {
//        "ISD": 1,
//        "AS": 2,
//        "CreditBalance": 10,
//        "Bandwidth": 100000,
//        "IsOutgoing": false,
//        "Timestamp": "2017-07-17 13:59:37.396791+00:00"
//      }
//  ]
//}

func (c *ASInfoController) ListASesConnectionsWithCredits(w http.ResponseWriter, r *http.Request) {
	if config.VirtualCreditEnable == false {
		c.NotFound(w, nil, systemDisabledError.Error())
		return
	}

	var response struct {
		ISD         addr.ISD                       // The ISD of this AS
		ASID        addr.AS                        // This AS
		Credits     int64                          // The current credits of this AS
		Connections []models.ConnectionWithCredits // List of connections and their costs / yields
	}

	vars := mux.Vars(r)
	ia, ok := vars["ia"]
	if !ok {
		c.BadRequest(w, nil, "missing ia parameter")
		return
	}

	requestingAS, err := models.FindASInfoByIA(ia)
	if err != nil {
		c.NotFound(w, err, ia+" not found")
		return
	}
	response.ISD = requestingAS.ISD
	response.ASID = requestingAS.ASID
	response.Credits = requestingAS.Credits

	connections, err := requestingAS.ListConnections()
	if err != nil {
		log.Printf("Error while retrieving list of ASes. ISD-AS: %v", requestingAS)
		c.BadRequest(w, err, "Error while retrieving list of ASes. ISD-AS: %v", requestingAS)
		return
	}
	response.Connections = connections

	b, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error during JSON marshaling. ISD-AS: %v, %v", requestingAS, err)
		c.Error500(w, err, "Error during JSON marshaling. ISD-AS: %v", requestingAS)
		return
	}
	fmt.Fprintln(w, string(b))
}

//
//	Checks if the requester has enough credits to open a ConnectionRequest.
//	If the requester has enough, he will pay the credit at this moment. If the request will
//	be not accepted later, the requester will get his Credits back.
//
//	The function will handle the response itself in case of an error. If everything was fine the
//	function returns nil.
//
func (c *ASInfoController) checkAndUpdateCredits(w http.ResponseWriter, r *http.Request,
	cr *ConnRequest) error {

	if config.VirtualCreditEnable == false {
		return systemDisabledError
	}

	as, err := models.FindASInfoByIA(cr.RequestIA)
	if err != nil {
		log.Printf("Error: Unkown AS: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error: Unknown AS: %v", r.Body)
		return err
	}

	var creditsNeeded = models.BandwidthToCredits(cr.Bandwidth)
	if (as.Credits - creditsNeeded) <= 0 {
		err = fmt.Errorf("error: Not enough credits to create a connection request! "+
			"You need %v, but have only %v", creditsNeeded, as.Credits)
		log.Printf("Info: Not enough credits! AS: %v, Request: %v, Error: %v", as, r.Body, err)
		c.BadRequest(w, err, "Info: Not enough credits! AS: %v, Request: %v", as, r.Body)
		return err
	}

	// Subtracting credits from AS
	if err := as.UpdateCurrency(-1 * creditsNeeded); err != nil {
		log.Printf("Error: Subtracting credits! AS: %v, Request: %v, Error: %v", as, r.Body, err)
		c.Error500(w, err, "Error: Subtracting credits! AS: %v, Request: %v", as, r.Body)
		return err
	}
	return nil
}

//
//	Handles the credit transaction for the requester and the target when a ConnectionResponse is
//	sent.
//
//	The function will handle the response itself in case of an error and returns the error.
//	If everything was fine, the function returns nil.
//
func (c *ASInfoController) checkAndUpdateCreditsAtResponse(w http.ResponseWriter, r *http.Request,
	cr *models.ConnRequest, reply ConnReply) error {

	if !config.VirtualCreditEnable == false {
		return systemDisabledError
	}

	as, _ := models.FindASInfoByIA(cr.RequestIA)
	credits := models.BandwidthToCredits(cr.Bandwidth)
	// If the connection is approved, add credits to the responding AS
	if cr.Status == models.Approved {
		otherAS, err := models.FindASInfoByIA(cr.RespondIA)
		if err != nil {
			log.Printf("Error finding the RespondIA. Request ID: %v RequestIA: %v, RespondIA: %v, %v",
				reply.RequestID, reply.RequestIA, reply.RespondIA, err)
			c.Error500(w, err, "Error finding the RespondIA. Request ID: %v RequestIA: %v, RespondIA: %v",
				reply.RequestID, reply.RequestIA, reply.RespondIA)
			return err
		}
		if err := otherAS.UpdateCurrency(credits); err != nil {
			log.Printf("Error: Adding credits! OtherAS: %v, Request: %v, Error: %v", otherAS, r.Body, err)
			c.Error500(w, err, "Error: Adding credits! OtherAS: %v, Request: %v", otherAS, r.Body)
			return err
		}
		// If the connection request is denied, release the reserved credits for the connection
	} else if cr.Status != models.Pending {
		if err := as.UpdateCurrency(credits); err != nil {
			log.Printf("Error: Re-adding credits! ThisAS: %v, Request: %v, Error: %v", as, r.Body, err)
			c.Error500(w, err, "Error: Re-adding credits! ThisAS: %v, Request: %v", as, r.Body)
			return err
		}
	}
	return nil
}

//
//	If an error occurs while handling the ConnectionResponse, this function will rollback the change
//	in credits.
//
//	If everything was fine, the function returns nil.
//
func (c *ASInfoController) rollBackCreditUpdate(w http.ResponseWriter, r *http.Request, cr *ConnRequest) {
	if !config.VirtualCreditEnable {
		return
	}

	as, err := models.FindASInfoByIA(cr.RequestIA)
	if err != nil {
		log.Printf("Error: Unkown AS: %v, %v", r.Body, err)
		c.BadRequest(w, err, "Error: Unknown AS: %v", r.Body)
		return
	}

	creditsNeeded := models.BandwidthToCredits(cr.Bandwidth)
	// Roll back UpdateCurrency changes
	if err := as.UpdateCurrency(creditsNeeded); err != nil {
		log.Printf("Critical error: Can't roll back UpdateCurrency changes! "+
			"Credits: %v, AS: %v, Request: %v, Error: %v", creditsNeeded, as, r.Body, err)
	}
}
