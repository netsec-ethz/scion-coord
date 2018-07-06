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
	"io/ioutil"
	"log"
	"net/http"
	"strings"

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
func (s *SCIONLabASController) checkAuthorization(r *http.Request) (ia addr.IA, err error) {
	apIA := r.URL.Query().Get("scionLabAP")
	if len(apIA) == 0 {
		err = errors.New("scionLabAP parameter missing")
		return
	}
	ia, err = addr.IAFromString(apIA)
	if err != nil {
		ia, err = addr.IAFromFileFmt(apIA, false)
		if err != nil {
			err = fmt.Errorf("%v is not a valid SCION IA", apIA)
			return
		}
	}
	// ensure apIA is always non file format:
	apIA = ia.String()

	ases, err := s.ownedASes(r)
	if err == nil {
		if _, ourAS := ases[apIA]; ourAS {
			return
		}
	}
	err = fmt.Errorf("the Attachment Point %v does not belong to the specified account", apIA)
	return
}

// List of all ASes belonging to the account
func (s *SCIONLabASController) ownedASes(r *http.Request) (ases map[string]struct{}, err error) {
	vars := mux.Vars(r)
	accountID := vars["account_id"]
	asesList, err := models.FindSCIONLabASesByAccountID(accountID)
	ases = make(map[string]struct{})
	for _, as := range asesList {
		ases[as] = struct{}{}
	}
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
	log.Printf("API Call for getUpdatesForAP = %v", r.URL.Query())
	apIA, err := s.checkAuthorization(r)
	if err != nil {
		s.Forbidden(w, err, "The account is not authorized for this AP")
		return
	}

	as, err := models.FindSCIONLabASByIAInt(apIA.I, apIA.A)
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
	var cnsCreateResp []APConnectionInfo
	var cnsUpdateResp []APConnectionInfo
	var cnsRemoveResp []APConnectionInfo
	for _, cn := range cnInfos {
		cnInfo := APConnectionInfo{
			ASID:      utility.IAStringStandard(as.ISD, cn.NeighborAS),
			IsVPN:     cn.IsVPN,
			VPNUserID: vpnUserID(cn.NeighborUser, cn.NeighborAS),
			IP:        cn.NeighborIP,
			UserPort:  cn.NeighborPort,
			APPort:    cn.LocalPort,
			APBRID:    cn.BRID,
		}
		switch cn.Status {
		case models.Create:
			cnsCreateResp = append(cnsCreateResp, cnInfo)
		case models.Update:
			cnsUpdateResp = append(cnsUpdateResp, cnInfo)
		case models.Remove:
			cnsRemoveResp = append(cnsRemoveResp, cnInfo)
		}
	}
	resp := map[string]map[string][]APConnectionInfo{
		apIA.FileFmt(false): {
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
	log.Printf("getUpdatesForAP will return: %v", string(b))
	fmt.Fprintln(w, string(b))
}

type attachedASAckMessage struct {
	IA      string
	Success bool
}

type rejectedAS struct {
	IA     string
	AP     string
	action string
}

// API end-point to mark the provided SCIONLabASes as Created, Updated or Removed
// An example request to this API may look like the following:
// {"1-7":
//        {"Created":[],
//         "Removed":[],
//         "Updated":[{"IA": "1-1020", "success": true}, {"IA": "1-1020", "success": false}]
//        }
// }
// If successful, the API will return an empty JSON response with HTTP code 200.
func (s *SCIONLabASController) ConfirmUpdatesFromAP(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Call for ConfirmUpdatesFromAP")
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body of HTTP request. Error: %v \nBody: %v", r.Body, err)
		s.BadRequest(w, err, "Error reading request body")
		return
	}
	body := string(bodyBytes)

	var updateLists map[string]map[string][]attachedASAckMessage
	decoder := json.NewDecoder(strings.NewReader(body))
	if err := decoder.Decode(&updateLists); err != nil {
		log.Printf("Error decoding JSON: %v, %v", err, body)
		s.BadRequest(w, err, "Error decoding JSON")
		return
	}
	ownedASes, err := s.ownedASes(r)
	if err != nil {
		s.BadRequest(w, err, "Error looking up owned ASes")
	}

	var failedConfirmations []string
	var rejectedIAs []rejectedAS
	for ia, event := range updateLists {
		IA, err := addr.IAFromString(ia)
		if err != nil {
			IA, err = addr.IAFromFileFmt(ia, false)
			if err != nil {
				err = fmt.Errorf("%v is not a valid SCION IA", ia)
				return
			}
		}
		// ensure apIA is always non file format:
		ia = IA.String()

		_, isAuthorized := ownedASes[ia]
		if !isAuthorized {
			log.Printf("Unauthorized updates from AS %v", ia)
		}
		as, err := models.FindSCIONLabASByIAInt(IA.I, IA.A)
		if err != nil {
			log.Printf("Error finding AS %v when processing confirmations: %v", ia, err)
		}
		if !isAuthorized || err != nil {
			for _, cns := range event {
				for _, attachedAS := range cns {
					ia := attachedAS.IA
					failedConfirmations = append(failedConfirmations, ia)
				}
			}
			continue
		}

		for action, cns := range event {
			var successIAs []string
			for _, conn := range cns {
				if conn.Success {
					successIAs = append(successIAs, conn.IA)
				} else {
					rejectedIAs = append(rejectedIAs, rejectedAS{IA: conn.IA, AP: ia, action: action})
				}
			}
			failedConfirmations = append(failedConfirmations, s.processConfirmedUpdatesFromAP(as, action, successIAs)...)
		}
	}
	failedConfirmations = append(failedConfirmations, s.processRejectedUpdatesFromAP(rejectedIAs)...)
	if len(failedConfirmations) > 0 {
		log.Printf("ERROR processing confirmations for the following ASes: %v", failedConfirmations)
		s.Error500(w, nil, "Error processing confirmations for the following ASes: %v",
			failedConfirmations)
		return
	}
	fmt.Fprintln(w, "{}")
}

// Updates the relevant DB tables based on the received confirmations from the SCIONLab AP and sends
// out confirmation emails
func (s *SCIONLabASController) processConfirmedUpdatesFromAP(apAS *models.SCIONLabAS, action string, cns []string) []string {
	log.Printf("action = %v, cns = %v", action, cns)
	type emailConfirmation struct {
		user   string
		IA     string
		action string
	}
	var failedConfirmations []string
	var successEmails []emailConfirmation
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
			log.Printf("Error finding SCIONLabAS with AS ID %v: %v", IA.A, err)
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		asCns, err := as.GetJoinConnectionInfoToAS(apAS.IAString())
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
				ia, apAS.IAString(), len(workingSet))
			failedConfirmations = append(failedConfirmations, ia)
			continue
		}
		cnInfo := workingSet[0]
		switch action {
		case CREATED, UPDATED:
			cnInfo.Status = models.Active
		case REMOVED:
			if cnInfo.IsCurrentConnection() {
				cnInfo.Status = models.Inactive
				cnInfo.BRID = 0 // Set BRID to 0 for inactive connections
			} else {
				// this means to remove the connection entry but don't update the AS status
				err = as.DeleteConnectionFromDB(&cnInfo)
				if err != nil {
					log.Printf("Error removing connection between AS %v and AP %v: %v", ia, apAS.IAString(), err)
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
				log.Printf("Error updating database tables for AS %v: %v", as.IAString(), err)
				failedConfirmations = append(failedConfirmations, ia)
				continue
			}
		} else {
			// just checking for consistency
			if action != REMOVED {
				// logic error! print failed assertion but don't quit this update
				log.Printf("Logic error confirming updates for AS %v to AP %v. The connection is inactive but the action %v != REMOVED",
					ia, apAS.IAString(), action)
			}
		}
		successEmails = append(successEmails, emailConfirmation{as.UserEmail, as.IAString(), action})
	}
	for _, e := range successEmails {
		if err := sendConfirmationEmail(e.user, e.IA, e.action); err != nil {
			log.Printf("Error sending email confirmation to user %v: %v", e.user, err)
		}
	}
	return failedConfirmations
}

// processRejectedUpdatesFromAP will receive a list of AS with rejected updates,
// will notify the ScionLab administrators, and remove the pending change.
func (s *SCIONLabASController) processRejectedUpdatesFromAP(rejections []rejectedAS) []string {
	var failedNotifications []string
	// for each rejected AS, send an email to the admin and user
	for _, rejectedAS := range rejections {
		IA, err := addr.IAFromString(rejectedAS.IA)
		if err != nil {
			log.Printf("Error converting IA (%v) to its components: %v", rejectedAS.IA, err)
			failedNotifications = append(failedNotifications, rejectedAS.IA)
			continue
		}
		// the original IA may only be used to communicate to the user, as the real
		// one may be re-attached to a different AP, and thus, different. The ASID is the same though
		originalIA := rejectedAS.IA
		as, err := models.FindSCIONLabASByASID(IA.A)
		if err != nil {
			log.Printf("Error finding SCIONLabAS with AS ID %v: %v", IA.A, err)
			failedNotifications = append(failedNotifications, originalIA)
			continue
		}
		IA, err = addr.IAFromString(rejectedAS.AP)
		if err != nil {
			log.Printf("Error converting IA (%v) to its components: %v", rejectedAS.AP, err)
			failedNotifications = append(failedNotifications, originalIA)
			continue
		}
		ap, err := models.FindSCIONLabASByASID(IA.A)
		if err != nil {
			log.Printf("Error finding SCIONLabAS with AS ID %v: %v", IA.A, err)
			failedNotifications = append(failedNotifications, originalIA)
			continue
		}

		err = sendRejectedEmail(as.UserEmail, originalIA, rejectedAS.action, rejectedAS.AP)
		if err != nil {
			log.Printf("Error sending email about rejected AS old IA: %v, new IA: %v: %v", originalIA, as.IA(), err)
			failedNotifications = append(failedNotifications, originalIA)
			continue
		}

		asCns, err := ap.GetRespondConnectionInfoToAS(as.IA().A)
		if err != nil {
			log.Printf("Error finding the connection to SCIONLabAS %v: %v", as.IA(), err)
			failedNotifications = append(failedNotifications, originalIA)
			continue
		}
		log.Printf("[DEBUG ConfirmUpdatesFromAP] connections between AP %v and user AS %v: %v", ap.IAString(), as.IA().A, asCns)
		// now, clear the rejected AS, as the AP went out of sync with the Coordinator
		for _, cn := range asCns {
			if cn.Status != models.Create && cn.Status != models.Update && cn.Status != models.Remove {
				// if we don't have pending actions, skip completely
				continue
			}
			err = models.DeleteConnection(cn.ID)
			if err != nil {
				log.Printf("ERROR removing rejected connection. UserAS: %s, AP: %s, action: %s", rejectedAS.IA, rejectedAS.AP, rejectedAS.action)
				failedNotifications = append(failedNotifications, rejectedAS.IA)
			}
			break // only one connection could have been rejected. We just processed it, so get out of here
		}

		// fix the status of the AS entry, if needed:
		if as.Status == models.Update || as.Status == models.Remove {
			// only case where a rejected connection could keep the Status out of sync
			cns, err := as.GetJoinConnectionInfo()
			if err != nil {
				log.Printf("ERROR removing rejected connection, get connections to reset AS Status for AS %s: %v", rejectedAS.IA, err)
				failedNotifications = append(failedNotifications, rejectedAS.IA)
				continue
			}
			switch len(cns) {
			case 0:
				as.Status = models.Inactive
			case 1:
				as.Status = cns[0].Status
			default:
			}
			if err = as.Update(); err != nil {
				log.Printf("ERROR removing rejected connection. Updating status of user AS failed for %s: %v", rejectedAS.IA, err)
				continue
			}
		}
	}

	return failedNotifications
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
		HostAddress string
		Message     string
	}{
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		HostAddress: config.HTTPHostAddress,
		Message:     message,
	}
	log.Printf("Sending confirmation email to user %v.", userEmail)
	return email.ConstructFromTemplateAndSend("as_status.html", subject, data, "as-update", userEmail, false)
}

// sends an email notifying of a failure to synchronize the attachment point with the user AS.\
// Also notifies an admin in NetSec
func sendRejectedEmail(userEmail string, userIA, action, attachmentPointIA string) error {
	user, err := models.FindUserByEmail(userEmail)
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("[SCIONLab] Could not complete request for %s", userIA)
	data := struct {
		FirstName   string
		LastName    string
		HostAddress string
		AS          string
		Operation   string
		AP          string
	}{
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		HostAddress: config.HTTPHostAddress,
		AS:          userIA,
		Operation:   action,
		AP:          attachmentPointIA,
	}
	return email.ConstructFromTemplateAndSend("as_failure.html", subject, data, "as-rejection", userEmail, true)
}
