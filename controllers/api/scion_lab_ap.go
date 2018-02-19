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
	"io/ioutil"
	"log"
	"net/http"
	"strings"

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
	ACTIVE  = "ACTIVE"
)

// The struct used for API calls between scion-coord and SCIONLab APs
// TODO(mlegner): Change field names here and in the `update_gen.py` to reflect new conventions
type APConnectionInfo struct {
	ASID      string // ISD-AS of the AS
	IsVPN     bool   // is this a VPN connection
	VPNUserID string // user identifier used for VPN, currently the user's email + ASID
	UserIP    string // IP address of the SCIONLab AS
	UserPort  uint16 // port number of the AS connecting to the AP
	APPort    uint16 // port number at the AP
	APBRID    uint16 // ID of the border router at the AP
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
	apIA, err := checkAuthorization(r, r.URL.Query().Get("scionLabAP"))
	if err != nil {
		s.Forbidden(w, err, "The account is not authorized for this AP")
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
			UserIP:    cn.NeighborIP,
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

type emailConfirmation struct {
	user   string
	IA     string
	action string
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
	ownedASes, err := ownedASes(r)
	if err != nil {
		s.BadRequest(w, err, "Error looking up owned ASes")
		return
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
		// ensure ia is always non file format:
		ia = IA.String()
		var as *models.SCIONLabAS
		_, isAuthorized := ownedASes[ia]
		if !isAuthorized {
			log.Printf("Unauthorized updates from AS %v", ia)
		} else {
			as, err = models.FindSCIONLabASByIAInt(IA.I, IA.A)
			if err != nil {
				log.Printf("Error finding AS %v when processing confirmations: %v", ia, err)
			}
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
			if err = as.UpdateASAndConnectionFromJoinConnInfo(&cnInfo); err != nil {
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
				continue
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
			err = models.DeleteConnectionFromDB(cn.ID)
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

// GetConnectionsForAP will return a JSON with the connections for an AP as seen by the Coordinator
// Example of returned message:
// {
//     "17-ffaa:0:1107": {
//         "connections": [
//         {
//             "ASID": "17-ffaa:1:14",
//             "IsVPN": true,
//             "VPNUserID": "user@example.com_ffaa_1_14",
//             "UserIP": "10.0.8.42",
//             "UserPort": 50000,
//             "APPort": 50053,
//             "APBRID": 5
//         }
//         ]
//     }
// }
func (s *SCIONLabASController) GetConnectionsForAP(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Call for GetConnectionsForAP ----------------- BEGIN ----------------- %v",
		r.URL.Query())
	apIAparam := r.URL.Query().Get("scionLabAP")
	// seconds since Epoch filtering the connections. Only the older than that ones will be sent:
	cutoff := utility.GetTimeCutoff(r)
	apIA, err := checkAuthorization(r, apIAparam)
	if err != nil {
		s.Forbidden(w, err, "The account is not authorized for this AP")
		return
	}
	ap, err := models.FindSCIONLabASByIAInt(apIA.I, apIA.A)
	if err != nil {
		log.Printf("Error looking up the AS %v: %v", apIA, err)
		s.Error500(w, err, "Error looking up SCIONLab AS from DB")
		return
	}
	cns, err := ap.GetRespondConnections()
	if err != nil {
		log.Printf("Error looking up connections for AS %v: %v", apIA, err)
		s.Error500(w, err, "Error looking up SCIONLab ASes from DB")
		return
	}
	// var conns []APConnectionInfo
	conns := []APConnectionInfo{}
	for _, cn := range cns {
		if cn.RespondStatus != models.Active &&
			cn.RespondStatus != models.Create &&
			cn.RespondStatus != models.Update {
			// not pending to add or already active -> don't send info
			continue
		}
		if cn.Updated.Unix() > cutoff {
			continue
		}
		userAS := cn.GetJoinAS()
		cnInfo := APConnectionInfo{
			ASID:      userAS.IAString(),
			IsVPN:     cn.IsVPN,
			VPNUserID: vpnUserID(userAS.UserEmail, userAS.ASID),
			UserIP:    cn.JoinIP,
			UserPort:  userAS.GetPortNumberFromBRID(cn.JoinBRID),
			APPort:    ap.GetPortNumberFromBRID(cn.RespondBRID),
			APBRID:    cn.RespondBRID,
		}
		conns = append(conns, cnInfo)
	}
	resp := map[string]map[string]interface{}{
		apIA.FileFmt(false): {
			"connections": conns,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON marshaling: %v", err)
		s.Error500(w, err, "Error during JSON marshaling")
		return
	}
	log.Printf("getUpdatesForAP will return: %v", string(b))
	log.Printf("API Call for GetConnectionsForAP ----------------- END -----------------")
	fmt.Fprintln(w, string(b))
}

// SetConnectionsForAP receives the connections an AP has and flags them as such in the Coordinator
// In case the belief of the AP differs to that of the Coordinator, the AP will be notified. Then
// the AP could get the belief of the Coordinator via a different backend call
// Example of request:
// {
//     "17-ffaa:0:1107": {
//       "connections": [
//         {
//           "UserASID": "17-ffaa:1:14",
//           "UserIP": "10.0.8.42",
//           "UserPort": 50000,
//           "APIP": 10.0.8.1,
//           "APPort": 50004,
//           "APBRID": 5
//           "VPNUserID": "user@example.com_ffaa_1_14",
//         }
//       ]
//     }
//   }
func (s *SCIONLabASController) SetConnectionsForAP(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Call for SetConnectionsForAP ----------------- BEGIN -----------------")
	type IssueInAP struct {
		ShouldTryAgain    bool
		CriticalError     string
		FailedASesReasons map[string]string
	}
	CreateIssueInAP := func() *IssueInAP {
		return &IssueInAP{FailedASesReasons: make(map[string]string)}
	}
	// Response to the AP when this call is finished. Not being in the map means all okay:
	type ResponseToAP map[string]*IssueInAP
	// don't consider connections in DB modified after the cutoff:
	cutoff := utility.GetTimeCutoff(r)
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body of HTTP request. Error: %v \nBody: %v", r.Body, err)
		s.BadRequest(w, err, "Error reading request body")
		return
	}
	body := string(bodyBytes)
	var allStatusMap map[string]map[string][]APConnectionInfo
	decoder := json.NewDecoder(strings.NewReader(body))
	if err := decoder.Decode(&allStatusMap); err != nil {
		log.Printf("Error decoding JSON: %v, %v", err, body)
		s.BadRequest(w, err, "Error decoding JSON")
		return
	}
	ownedASes, err := ownedASes(r)
	if err != nil {
		s.BadRequest(w, err, "Error looking up owned ASes")
		return
	}

	var sendToAdminMessages []string
	var successEmails []emailConfirmation
	response := make(ResponseToAP)
	for apIAStr, reportedStatus := range allStatusMap {
		setCriticalError := func(errorMsg string) {
			if response[apIAStr] == nil {
				response[apIAStr] = CreateIssueInAP()
			}
			response[apIAStr].CriticalError = errorMsg
		}
		setUserASError := func(ia, msg string) {
			if response[apIAStr] == nil {
				response[apIAStr] = CreateIssueInAP()
			}
			response[apIAStr].ShouldTryAgain = true
			response[apIAStr].FailedASesReasons[ia] = msg
		}
		apIA, err := addr.IAFromString(apIAStr)
		if err != nil {
			apIA, err = addr.IAFromFileFmt(apIAStr, false)
			if err != nil {
				err = fmt.Errorf("%v is not a valid SCION IA", apIAStr)
				setCriticalError(err.Error())
				continue
			}
		}
		// ensure apIA is always non file format:
		apIAStr = apIA.String()
		log.Printf("[DEBUG] IA: %v, status: %v", apIAStr, reportedStatus)
		reportedConnections := reportedStatus["connections"]
		var ap *models.SCIONLabAS
		_, isAuthorized := ownedASes[apIAStr]
		if !isAuthorized {
			log.Printf("Unauthorized updates from AS %v", apIAStr)
		} else {
			ap, err = models.FindSCIONLabASByIAInt(apIA.I, apIA.A)
			if err != nil {
				log.Printf("[ERROR] Error finding AS %v when processing confirmations: %v", apIAStr, err)
			}
		}
		if !isAuthorized || err != nil {
			var reason string
			if !isAuthorized {
				reason = "Not authorized"
			} else {
				reason = fmt.Sprintf("Cannot obtain AS: %v", err)
			}
			var ias []string
			for _, c := range reportedConnections {
				ias = append(ias, c.ASID)
			}
			msg := fmt.Sprintf("Could not process set connections from AP %v. Reason: %v. Affected user ASes: %v",
				apIAStr, reason, ias)
			sendToAdminMessages = append(sendToAdminMessages, msg)
			setCriticalError(msg)
			continue
		}
		// all received connections for this AP, as a map:
		fromAP := make(map[addr.AS][]APConnectionInfo)
		for _, c := range reportedConnections {
			ia, err := addr.IAFromString(c.ASID)
			if err != nil {
				msg := fmt.Sprintf("[ERROR] String (%v) does not parse to IA: %v", c.ASID, err)
				log.Print(msg)
				setUserASError(c.ASID, msg)
				continue
			}
			fromAP[ia.A] = append(fromAP[ia.A], c)
		}

		// find the pending connections in the DB and the AP's received status,
		// and change the status accordingly
		cnsInDB, err := ap.GetRespondConnections()
		if err != nil {
			msg := fmt.Sprintf("[ERROR] Error looking up connections for AS %v: %v", apIAStr, err)
			log.Print(msg)
			setCriticalError(msg)
			continue
		}
		// find the pending and active connections in the received ones:
		cnInfosInDB := make(map[string][]APConnectionInfo)
		for _, cnInDB := range cnsInDB {
			if cnInDB.Updated.Unix() > cutoff {
				// ignore too new connections
				continue
			}
			// if the connection status in AP's side is not pending, skip
			var actionString string
			switch cnInDB.RespondStatus {
			case models.Create:
				actionString = CREATED
			case models.Update:
				actionString = UPDATED
			case models.Remove:
				actionString = REMOVED
			case models.Active:
				actionString = ACTIVE
			default:
				continue
			}
			userAS := cnInDB.GetJoinAS()
			userASIA := userAS.IAString()
			apCnInfo := APConnectionInfo{
				ASID:      userASIA,
				IsVPN:     cnInDB.IsVPN,
				VPNUserID: vpnUserID(userAS.UserEmail, userAS.ASID),
				UserIP:    cnInDB.JoinIP,
				UserPort:  userAS.GetPortNumberFromBRID(cnInDB.JoinBRID),
				APPort:    ap.GetPortNumberFromBRID(cnInDB.RespondBRID),
				APBRID:    cnInDB.RespondBRID,
			}
			cnInfosInDB[userASIA] = append(cnInfosInDB[userASIA], apCnInfo)
			cnArr := fromAP[userAS.ASID]
			foundPendingInReported := false
			for _, reportedCn := range cnArr {
				if reportedCn == apCnInfo {
					foundPendingInReported = true
					break
				}
			}
			origStatus := cnInDB.RespondStatus
			if foundPendingInReported {
				cnInDB.RespondStatus = models.Active
			} else if origStatus == models.Remove {
				if cnInDB.IsCurrentConnection() {
					cnInDB.RespondStatus = models.Inactive
					cnInDB.JoinBRID = 0 // Set join BRID to 0 for inactive connections
				} else {
					err := models.DeleteConnectionFromDB(cnInDB.ID)
					if err != nil {
						msg := fmt.Sprintf("[ERROR] Error removing connection between AP %v and AS %v: %v",
							apIA, userASIA, err)
						log.Print(msg)
						setUserASError(userASIA, msg)
						continue
					}
				}
			} else {
				// this is a not found connection that is active or pending to create or update. Complain
				msg := fmt.Sprintf("[ERROR] Connection present in DB but not in AP. Data: "+
					"from AP %v to ASID %v, user email %v, DB id %d, updated on %v, to %v",
					apIAStr, userASIA, userAS.UserEmail, cnInDB.ID, cnInDB.Updated, origStatus)
				log.Print(msg)
				setUserASError(userASIA, msg)
				sendToAdminMessages = append(sendToAdminMessages, msg)
				continue
			}
			// cnInDB was found in the AP, or not found but pending to remove (both cases okay).
			if cnInDB.IsCurrentConnection() {
				if origStatus != models.Active {
					// If the pending connection is the current one from the user AS to the AP,
					// then update the user AS status:
					userAS.Status = cnInDB.RespondStatus
					if err = userAS.UpdateASAndConnection(cnInDB); err != nil {
						msg := fmt.Sprintf("[ERROR] Cannot update AS and connection for AS %v: %v",
							userAS.IAString(), err)
						log.Print(msg)
						setUserASError(userASIA, msg)
						err = sendRejectedEmail(userAS.UserEmail, userASIA, actionString, apIAStr)
						if err != nil {
							log.Printf("[ERROR] Could not send email to user %v about failed sync between AP %v and user AS %v",
								userAS.UserEmail, apIAStr, userASIA)
						}
						continue
					}
					successEmails = append(successEmails, emailConfirmation{
						user:   userAS.UserEmail,
						IA:     userASIA,
						action: actionString})
				}
			} else {
				if origStatus != models.Remove {
					// logic error! print failed assertion but don't quit this update
					msg := fmt.Sprintf("[ERROR] Logic error setting connections for AP %v to user AS %v. "+
						"The connection is inactive but the action %v != REMOVED",
						apIAStr, userAS.IAString(), origStatus)
					log.Print(msg)
					sendToAdminMessages = append(sendToAdminMessages, msg)
					continue
				}
			}
		} // for each connection in DB
		// check that all the received connections exist as such in the DB
		for _, reportedConn := range reportedConnections {
			foundInDB := false
			for _, c := range cnInfosInDB[reportedConn.ASID] {
				if c == reportedConn {
					foundInDB = true
					break
				}
			}
			if !foundInDB {
				msg := fmt.Sprintf("A reported connection was not found in the DB. AP: %v user AS: %v, full APConnectionInfo: %v",
					apIAStr, reportedConn.ASID, reportedConn)
				log.Print(msg)
				sendToAdminMessages = append(sendToAdminMessages, msg)
				setUserASError(reportedConn.ASID, msg)
			}
		}
	} // for each AP,status
	for _, e := range successEmails {
		if err := sendConfirmationEmail(e.user, e.IA, e.action); err != nil {
			msg := fmt.Sprintf("Cannot send confirmation email to user %v about IA %v for action %v. Error is: %v",
				e.user, e.IA, e.action, err)
			log.Print(msg)
			sendToAdminMessages = append(sendToAdminMessages, msg)
		}
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		msg := fmt.Sprintf("[ERROR] Cannot serialize response to SetConnections to JSON: %v", err)
		log.Print(msg)
		sendToAdminMessages = append(sendToAdminMessages, msg)
		responseJSON = []byte("{}")
	}
	log.Printf("[DEBUG] Response to AP: %v\n", string(responseJSON))
	if len(sendToAdminMessages) > 0 {
		log.Printf("[DEBUG] Messages to the admins: %v", sendToAdminMessages)
		data := struct {
			ErrorMessages string
		}{
			ErrorMessages: strings.Join(sendToAdminMessages, "\n"),
		}
		err = email.ConstructFromTemplateAndSendToAdmins("setconnections_failed.html",
			"FAILED SetConnections", data, "")
		if err != nil {
			log.Printf("[ERROR] Error sending email: %v", err)
		}
	}
	log.Printf("API Call for SetConnectionsForAP ----------------- END -----------------")
	fmt.Fprintln(w, string(responseJSON))
}
