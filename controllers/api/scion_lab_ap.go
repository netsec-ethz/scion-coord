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
	"fmt"
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/email"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
)

// TODO(mlegner): As the BR ID is now determined in coord, the `update_gen.py` must be adjusted

// Acknowledgments for the performed operations by an SCIONLab AP
const (
	CREATED = "Created"
	UPDATED = "Updated"
	REMOVED = "Removed"
)

// The struct used for API calls between scion-coord and SCIONLab APs
// TODO(mlegner): Change field names here and in the `update_gen.py` to reflect new conventions
type APConnectionInfo struct {
	ASID         string // ISD-AS of the AS
	IsVPN        bool
	RemoteIAPort int    // port number of the remote SCIONLab AP being connected to
	UserEmail    string // User identifier used for VPN, currently the user's email
	VMIP         string // VMIP address of the SCIONLab AS
	RemoteBR     string // The name of the remote border router
}

// API end-point for the SCIONLab APs to query actions to be done for users' SCIONLabASes.
// An example response to this API may look like the following:
// {"1-7":
//        {"Create":[],
//         "Remove":[],
//         "Update":[{"ASID":"1-1020",
//                    "IsVPN":true,
//                    "RemoteIAPort":50053,
//                    "UserEmail":"user@example.com",
//                    "VMIP":"10.0.8.42",
//                    "RemoteBR":"br1-5-5"}]
//        }
// }
func (s *SCIONLabASController) GetUpdatesForAP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside GetUpdatesForAP = %v", r.URL.Query())
	apIA := r.URL.Query().Get("scionLabAS")
	if len(apIA) == 0 {
		s.BadRequest(w, nil, "scionLabAS parameter missing")
		return
	}

	cns, err := models.FindRespondConnectionInfoByIA(apIA)
	if err != nil {
		log.Printf("Error looking up connections for AS %v: %v", apIA, err)
		s.Error500(w, err, "Error looking up SCIONLab ASes from DB")
		return
	}
	cnsCreateResp := []APConnectionInfo{}
	cnsUpdateResp := []APConnectionInfo{}
	cnsRemoveResp := []APConnectionInfo{}
	for _, cn := range cns {
		cnInfo := APConnectionInfo{
			ASID:         utility.IAString(cn.NeighborISD, cn.NeighborAS),
			IsVPN:        cn.IsVPN,
			UserEmail:    cn.NeighborUser,
			VMIP:         cn.NeighborIP,
			RemoteIAPort: cn.RemotePort,
			RemoteBR:     utility.BRString(apIA, cn.BRID),
		}
		switch cn.Status {
		case models.CREATE:
			cnsCreateResp = append(cnsCreateResp, cnInfo)
		case models.UPDATE:
			cnsUpdateResp = append(cnsUpdateResp, cnInfo)
		case models.REMOVE:
			cnsRemoveResp = append(cnsRemoveResp, cnInfo)
		}
	}
	resp := map[string]map[string][]APConnectionInfo{
		apIA: {
			"Create": cnsCreateResp,
			"Update": cnsUpdateResp,
			"Remove": cnsRemoveResp,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON marshaling: %v", err)
		s.Error500(w, err, "Error during JSON marshaling")
		return
	}
	fmt.Fprintln(w, string(b))
}

// API end-point to mark the provided SCIONLabASes as Created, Updated or Removed
// An example request to this API may look like the following:
// {"1-7":
//        {"Created":[],
//         "Removed":[],
//         "Updated":[{"ASID":"1-1020",
//                     "RemoteIAPort":50053,
//                     "VMIP":"203.0.113.0",
//                     "RemoteBR":"br1-5-5"}]
//        }
// }
// If sucessful, the API will return an empty JSON response with HTTP code 200.
func (s *SCIONLabASController) ConfirmUpdatesFromAP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside ConfirmUpdatesFromAP")
	var UpdateLists map[string]map[string][]APConnectionInfo
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&UpdateLists); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.BadRequest(w, err, "Error decoding JSON")
		return
	}
	failedConfirmations := []string{}
	for ia, event := range UpdateLists {
		as, err := models.FindSCIONLabASByIAString(ia)
		if err != nil {
			log.Printf("Error finding AS %v when processing confirmations: %v", ia, err)
			for _, cns := range event {
				for _, cn := range cns {
					failedConfirmations = append(failedConfirmations, cn.ASID)
				}
			}
		}
		for action, cns := range event {
			failedConfirmations = append(failedConfirmations, s.processConfirmedUpdatesFromAP(
				as, action, cns)...)
		}
	}
	if len(failedConfirmations) > 0 {
		s.Error500(w, nil, "Error processing confirmations for the following ASes: %v",
			failedConfirmations)
		return
	}
	fmt.Fprintln(w, "{}")
}

// Updates the relevant DB tables based on the received confirmations from the SCIONLab AP and sends
// out confirmation emails
func (s *SCIONLabASController) processConfirmedUpdatesFromAP(apAS *models.SCIONLabAS, action string,
	cns []APConnectionInfo) []string {
	log.Printf("action = %v, cns = %v", action, cns)
	failedConfirmations := []string{}
	for _, cn := range cns {
		// find the connection to the SCIONLabAS
		slas, err := models.FindSCIONLabASByIAString(cn.ASID)
		if err != nil {
			log.Printf("Error finding SCIONLabAS %v: %v", cn.ASID, err)
			failedConfirmations = append(failedConfirmations, cn.ASID)
			continue
		}
		cn_db, err := slas.GetJoinConnectionInfoToAS(apAS.String())
		if err != nil {
			log.Printf("Error finding the connection to SCIONLabAS %v: %v", cn.ASID, err)
			failedConfirmations = append(failedConfirmations, cn.ASID)
			continue
		}
		switch action {
		case CREATED, UPDATED:
			br, err := utility.BRIDFromString(cn.RemoteBR)
			if err != nil {
				log.Printf("Error parsing the BR name: %v", err)
				failedConfirmations = append(failedConfirmations, cn.ASID)
				continue
			}
			if br != cn_db.BRID {
				log.Printf("Reported BR ID %v differs from stored value %v", br, cn_db.BRID)
				failedConfirmations = append(failedConfirmations, cn.ASID)
				continue
			}
			cn_db.Status = models.ACTIVE
		case REMOVED:
			cn_db.Status = models.INACTIVE
			cn_db.BRID = -1 // Set BRID to -1 for inactive connections
		default:
			log.Printf("Unsupported action \"%v\" for AS %v. User: %v", action, cn.ASID,
				cn_db.NeighborUser)
			failedConfirmations = append(failedConfirmations, cn.ASID)
			continue
		}
		slas.Status = cn_db.Status
		if err = slas.UpdateASAndConnection(&cn_db); err != nil {
			log.Printf("Error updating database tables for AS %v: %v", slas.String(), err)
			failedConfirmations = append(failedConfirmations, cn.ASID)
			continue
		}
		if err := sendConfirmationEmail(cn_db.NeighborUser, action); err != nil {
			log.Printf("Error sending email confirmation to user %v: %v",
				cn_db.NeighborUser, err)
		}

	}
	return failedConfirmations
}

// Function which sends confirmation emails to users
func sendConfirmationEmail(userEmail, action string) error {
	user, err := models.FindUserByEmail(userEmail)
	if err != nil {
		return err
	}

	var message string
	subject := "[SCIONLab] "
	switch action {
	case CREATED:
		message = "The infrastructure for your SCIONLab AS has been created. " +
			"You are now able to use the SCION network through your AS."
		subject += "AS creation request completed"
	case UPDATED:
		message = "The settings for your SCIONLab AS have been updated."
		subject += "AS update request completed"
	case REMOVED:
		message = "Your removal request has been processed. " +
			"All infrastructure for your SCIONLab AS has been removed."
		subject += "AS removal request completed"
	}

	data := struct {
		FirstName   string
		LastName    string
		HostAddress string
		Message     string
	}{user.FirstName, user.LastName, config.HTTP_HOST_ADDRESS, message}

	log.Printf("Sending confirmation email to user %v.", userEmail)
	if err := email.ConstructAndSend("as_status.html", subject, data, "as-update",
		userEmail); err != nil {
		return err
	}

	return nil
}
