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

	"errors"

	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/email"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
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
	ASID      string // ISD-AS of the AS
	IsVPN     bool   // is this a VPN connection
	VPNUserID string // user identifier used for VPN, currently the user's email + ASID
	IP        string // IP address of the SCIONLab AS
	UserPort  uint16 // port number of the AS connecting to the AP
	APPort    uint16 // port number at the AP
	APBRID    uint16 // ID of the border router at the AP
}

// Check if the account is the owner of the specified Attachment Point
func (s *SCIONLabASController) checkAuthorization(r *http.Request) (apIA string, err error) {
	log.Printf("API Call for getUpdatesForAP = %v", r.URL.Query())
	apIA = r.URL.Query().Get("scionLabAP")
	if len(apIA) == 0 {
		err = errors.New("scionLabAP parameter missing")
		return
	}

	ases, err := s.ownedASes(r)
	if err != nil {
		return
	}

	for _, as := range ases {
		if as == apIA {
			return
		}
	}
	err = fmt.Errorf("The Attachment Point %v does not belong to the specified account", apIA)
	return
}

// List of all ASes belonging to the account
func (s *SCIONLabASController) ownedASes(r *http.Request) (ases []string, err error) {
	vars := mux.Vars(r)
	accountID := vars["account_id"]
	ases, err = models.FindSCIONLabASesByAccountID(accountID)
	return
}

// API end-point for the SCIONLab APs to query actions to be done for users' SCIONLabASes.
// An example response to this API may look like the following:
// {"1-7":
//        {"Create":[],
//         "Remove":[],
//         "Update":[{"ASID":"1-1020",
//                    "IsVPN":true,
//                    "VPNUserID":"user@example.com_1020",
//                    "IP":"10.0.8.42",
//                    "UserPort":50000,
//                    "APPort":50053,
//                    "APBRID":5}]
//        }
// }
func (s *SCIONLabASController) GetUpdatesForAP(w http.ResponseWriter, r *http.Request) {
	apIA, err := s.checkAuthorization(r)
	if err != nil {
		s.Forbidden(w, err, "The account is not authorized for this AP")
		return
	}

	as, err := models.FindSCIONLabASByIAString(apIA)
	if err != nil {
		log.Printf("Error looking up the AS %v: %v", apIA, err)
		s.Error500(w, err, "Error looking up SCIONLab AS from DB")
		return
	}
	cnInfos, err := as.GetRespondConnectionInfo()
	if err != nil {
		log.Printf("Error looking up connections for AS %v: %v", apIA, err)
		s.Error500(w, err, "Error looking up SCIONLab ASes from DB")
		return
	}
	cnsCreateResp := []APConnectionInfo{}
	cnsUpdateResp := []APConnectionInfo{}
	cnsRemoveResp := []APConnectionInfo{}
	for _, cn := range cnInfos {
		cnInfo := APConnectionInfo{
			ASID:      utility.IAString(as.ISD, cn.NeighborAS),
			IsVPN:     cn.IsVPN,
			VPNUserID: s.vpnUserID(cn.NeighborUser, cn.NeighborAS),
			IP:        cn.NeighborIP,
			UserPort:  cn.NeighborPort,
			APPort:    cn.LocalPort,
			APBRID:    cn.BRID,
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
//         "Updated":["1-1020", "1-1023"]
//        }
// }
// If sucessful, the API will return an empty JSON response with HTTP code 200.
func (s *SCIONLabASController) ConfirmUpdatesFromAP(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Call for ConfirmUpdatesFromAP")

	var UpdateLists map[string]map[string][]string
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&UpdateLists); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.BadRequest(w, err, "Error decoding JSON")
		return
	}

	ownedASes, err := s.ownedASes(r)
	if err != nil {
		s.BadRequest(w, err, "Error looking up owned ASes")
	}

	failedConfirmations := []string{}
	for ia, event := range UpdateLists {
		isAuthorized := false
		for _, as := range ownedASes {
			if as == ia {
				isAuthorized = true
			}
		}
		if !isAuthorized {
			log.Printf("Unauthorized updates from AS %v", ia)
		}
		as, err := models.FindSCIONLabASByIAString(ia)
		if err != nil {
			log.Printf("Error finding AS %v when processing confirmations: %v", ia, err)
		}
		if !isAuthorized || err != nil {
			for _, cns := range event {
				for _, ia := range cns {
					failedConfirmations = append(failedConfirmations, ia)
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
	cns []string) []string {
	log.Printf("action = %v, cns = %v", action, cns)
	type emailConfirmation struct {
		user   string
		IA     string
		action string
	}
	failedConfirmations := []string{}
	emails := []emailConfirmation{}
	for _, ia := range cns {
		// find the connection to the SCIONLabAS. e.g. ia=1-1001
		IA, err := addr.IAFromString(ia)
		if err != nil {
			log.Printf("Error converting IA (%v) to its components: %v", ia, err)
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		as, err := models.FindSCIONLabASByASID(IA.A)
		if err != nil {
			log.Printf("Error finding SCIONLabAS %v: %v", ia, err)
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		asCns, err := as.GetJoinConnectionInfoToAS(apAS.IA())
		if err != nil {
			log.Printf("Error finding the connection to SCIONLabAS %v: %v", ia, err)
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		// for removed, the connection can be active or inactive, depending on whether this
		// is the last AP or not. For created and updated, the connection must be active
		activeCns := models.OnlyCurrentConnections(asCns)
		inactiveCns := models.OnlyNotCurrentConnections(asCns)
		var workingSet []models.ConnectionInfo
		if action == REMOVED && len(inactiveCns) == 1 {
			workingSet = inactiveCns
		} else {
			workingSet = activeCns
		}
		if len(workingSet) != 1 {
			// we've failed our axiom that there's only one active connection. Complain
			log.Printf("Error confirming updates for AS %v: we expected 1 connection to %v and found %v",
				ia, apAS.IA(), len(workingSet))
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		cnInfo := workingSet[0]
		switch action {
		case CREATED, UPDATED:
			cnInfo.Status = models.ACTIVE
		case REMOVED:
			if cnInfo.IsCurrentConnection() {
				cnInfo.Status = models.INACTIVE
				cnInfo.BRID = 0 // Set BRID to 0 for inactive connections
			} else {
				// this means to remove the connection entry but don't update the AS status
				err = as.DeleteConnectionFromDB(&cnInfo)
				if err != nil {
					log.Printf("Error removing connection between AS %v and AP %v: %v", ia, apAS.IA(), err)
					continue
				}
			}
		default:
			log.Printf("Unsupported action \"%v\" for AS %v. User: %v", action, ia, as.UserEmail)
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		if cnInfo.IsCurrentConnection() {
			as.Status = cnInfo.Status
			if err = as.UpdateASAndConnection(&cnInfo); err != nil {
				log.Printf("Error updating database tables for AS %v: %v", as.IA(), err)
				failedConfirmations = append(failedConfirmations, ia)
				continue
			}
		} else {
			// just checking for consistency
			if action != REMOVED {
				// logic error! print failed assertion but don't quit this update
				log.Printf("Logic error confirming updates for AS %v to AP %v. The connection is inactive but the action %v != REMOVED",
					ia, apAS.IA(), action)
			}
		}
		emails = append(emails, emailConfirmation{as.UserEmail, as.IA(), action})
	}
	for _, e := range emails {
		if err := sendConfirmationEmail(e.user, e.IA, e.action); err != nil {
			log.Printf("Error sending email confirmation to user %v: %v", e.user, err)
		}
	}
	return failedConfirmations
}

// Function which sends confirmation emails to users
func sendConfirmationEmail(userEmail, IA, action string) error {
	user, err := models.FindUserByEmail(userEmail)
	if err != nil {
		return err
	}

	var message string
	subject := "[SCIONLab] "
	switch action {
	case CREATED:
		message = fmt.Sprintf("The infrastructure for your SCIONLab AS %s has been created. "+
			"You are now able to use the SCION network through your AS.", IA)
		subject += "AS creation request completed"
	case UPDATED:
		message = fmt.Sprintf("The settings for your SCIONLab AS %s have been updated.", IA)
		subject += "AS update request completed"
	case REMOVED:
		message = fmt.Sprintf("Your removal request has been processed. "+
			"All infrastructure for your SCIONLab AS %s has been removed.", IA)
		subject += "AS removal request completed"
	}

	data := struct {
		FirstName   string
		LastName    string
		Protocol    string
		HostAddress string
		Message     string
	}{user.FirstName, user.LastName, config.HTTP_PROTOCOL, config.HTTP_HOST_ADDRESS, message}

	log.Printf("Sending confirmation email to user %v.", userEmail)
	if err := email.ConstructAndSend("as_status.html", subject, data, "as-update",
		userEmail); err != nil {
		return err
	}

	return nil
}
