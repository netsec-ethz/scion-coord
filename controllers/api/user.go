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
	"log"
	"net/http"

	"fmt"
	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
)

type vmInfo struct {
	VMStatus uint8
	VMText   string
	VMIP     string
	ShowIP   bool
	ShowVPN  bool
	RemoteIA string
	UserPort int
}

type buttonConfiguration struct {
	Text            string // Button text
	Class           string // CSS class of button
	Action          string // Action to be taken when clicked
	Hide            bool   // Remove button completely
	Disable         bool   // Disable button
	TooltipDisabled string // Tooltip showed when button is disabled
}

type uiButtons struct {
	Update   buttonConfiguration // Button to create or update VM
	Download buttonConfiguration // Button to re-download VM
	Remove   buttonConfiguration // Button to remove VM
}

type userPageData struct {
	User      user
	ActiveVMs []vmInfo
	VMsInfo   []vmInfo
	UIButtons []uiButtons
}

// generates the structs containing information about the user's VM and the
// configuration of UI buttons
func constructMeData(u user) (userPageData, error) {

	var me userPageData
	vmsInfo := []vmInfo{}
	activeVMs := []vmInfo{}
	allButtons := []uiButtons{}

	scionLabServers, err := models.GetAllSCIONLabServerIAs()
	if err != nil {
		return me, err
	}

	for _, server := range scionLabServers {

		vmInfo := vmInfo{
			RemoteIA: server.IA,
			UserPort: server.UserPort,
		}
		buttons := uiButtons{
			Update: buttonConfiguration{
				Text:            "Update and Download SCIONLab VM Configuration",
				Action:          "update",
				TooltipDisabled: "Updates are disabled as you have a pending request.",
				Disable:         true,
			},
			Download: buttonConfiguration{
				Text:            "Re-download my SCIONLab VM Configuration",
				Action:          "download",
				TooltipDisabled: "You currently do not have an active VM configuration.",
			},
			Remove: buttonConfiguration{
				Text:            "Remove my SCIONLab VM Configuration",
				Class:           "btn-danger",
				Action:          "remove",
				TooltipDisabled: "You currently do not have an active VM configuration.",
				Disable:         true,
			},
		}

		vm, err := models.FindSCIONLabVMByServerIAAndUserEmail(server.IA, u.Email)
		if err != nil {
			if err != orm.ErrNoRows {
				return me, err
			}
		} else {
			vmInfo.VMIP = vm.IP
			vmInfo.VMStatus = vm.Status
		}
		switch vmInfo.VMStatus {
		case INACTIVE:
			vmInfo.VMText = fmt.Sprintf("You currently do not have "+
				"an active VM configuration connected to AS %v.", server.IA)
			buttons.Update.Text = "Create and Download SCIONLab VM Configuration"
			buttons.Update.Disable = false
			buttons.Download.Hide = true
			buttons.Remove.Hide = true
		case ACTIVE:
			vmInfo.VMText = fmt.Sprintf("You currently have "+
				"an active VM configuration connected to AS %v.", server.IA)
			buttons.Update.Disable = false
			buttons.Remove.Disable = false
		case CREATE:
			vmInfo.VMText = fmt.Sprintf("You have a pending creation request for your SCIONLab VM connected to AS %v.", server.IA)
		case UPDATE:
			vmInfo.VMText = fmt.Sprintf("You have a pending update request for your SCIONLab VM connected to AS %v.", server.IA)
		case REMOVE:
			vmInfo.VMText = "Your SCIONLab VM configuration is currently scheduled for removal."
			buttons.Download.Disable = true
		}
		if vmInfo.VMStatus == ACTIVE || vmInfo.VMStatus == CREATE || vmInfo.VMStatus == UPDATE {
			if vm.IsVPN {
				vmInfo.ShowVPN = true
			} else {
				vmInfo.ShowIP = true
			}
		}

		vmsInfo = append(vmsInfo, vmInfo)
		allButtons = append(allButtons, buttons)
	}

	for _, vm := range vmsInfo {
		if vm.VMStatus == 1 {
			activeVMs = append(activeVMs, vm)
		}
	}

	me = userPageData{
		User:      u,
		VMsInfo:   vmsInfo,
		ActiveVMs: activeVMs,
		UIButtons: allButtons,
	}

	return me, nil
}

// generates the user-information struct to be used in dynamic HTML pages
func populateUserData(w http.ResponseWriter, r *http.Request) (u user, err error) {
	// get the current user session if present.
	// if not then, abort
	_, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		log.Printf("Error authenticating user: Not logged in")
		http.Error(w, "Error authenticating user: Not logged in", http.StatusForbidden)
		return
	}

	// retrieve the user via the email
	storedUser, err := models.FindUserByEmail(userSession.Email)
	if err != nil {
		return
	}

	u = user{
		Email:        storedUser.Email,
		FirstName:    storedUser.FirstName,
		LastName:     storedUser.LastName,
		IsAdmin:      storedUser.IsAdmin,
		Account:      storedUser.Account.Name,
		Organisation: storedUser.Account.Organisation,
		AccountID:    storedUser.Account.AccountId,
		Secret:       storedUser.Account.Secret,
	}

	return
}

// API function that generates all information necessary for displaying the user page
func (c *LoginController) UserInformation(w http.ResponseWriter, r *http.Request) {

	user, err := populateUserData(w, r)
	if err != nil {
		log.Println(err)
		c.Forbidden(err, w, r)
		return
	}

	me, err := constructMeData(user)
	if err != nil {
		c.Forbidden(err, w, r)
		log.Printf("Error when generating VM info and button configuration for user %v: %v",
			user.Email, err)
		return
	}

	c.JSON(&me, w, r)
}
