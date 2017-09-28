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

type meData struct {
	User      user
	VMInfo    vmInfo
	UIButtons uiButtons
}

// generates the structs containing information about the user's VM and the
// configuration of UI buttons
func populateVMStatusButtons(userEmail string) (vmInfo, uiButtons, error) {
	vmInfo := vmInfo{}
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

	vm, err := models.FindSCIONLabVMByUserEmail(userEmail)
	if err != nil {
		if err != orm.ErrNoRows {
			return vmInfo, buttons, err
		}
	} else {
		vmInfo.VMIP = vm.IP
		vmInfo.VMStatus = vm.Status
	}
	switch vmInfo.VMStatus {
	case INACTIVE:
		vmInfo.VMText = "You currently do not have an active SCIONLab VM."
		buttons.Update.Text = "Create and Download SCIONLab VM Configuration"
		buttons.Update.Disable = false
		buttons.Download.Hide = true
		buttons.Remove.Hide = true
	case ACTIVE:
		vmInfo.VMText = "You currently have an active SCIONLab VM."
		buttons.Update.Disable = false
		buttons.Remove.Disable = false
	case CREATE:
		vmInfo.VMText = "You have a pending creation request for your SCIONLab VM."
	case UPDATE:
		vmInfo.VMText = "You have a pending update request for your SCIONLab VM."
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

	return vmInfo, buttons, nil
}

// API function that generates all information necessary for displaying the user page
func (c *LoginController) Me(w http.ResponseWriter, r *http.Request) {
	// get the current user session if present.
	// if not then, abort
	_, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// retrieve the user via the email
	storedUser, err := models.FindUserByEmail(userSession.Email)
	if err != nil {
		c.Forbidden(err, w, r)
		return
	}

	user := user{
		Email:        storedUser.Email,
		FirstName:    storedUser.FirstName,
		LastName:     storedUser.LastName,
		Account:      storedUser.Account.Name,
		Organisation: storedUser.Account.Organisation,
		AccountID:    storedUser.Account.AccountId,
		Secret:       storedUser.Account.Secret,
	}
	vmInfo, buttons, err := populateVMStatusButtons(userSession.Email)
	if err != nil {
		c.Forbidden(err, w, r)
		log.Printf("Error when generating VM info and button configuration for user %v: %v",
			userSession.Email, err)
		return
	}

	me := meData{
		User:      user,
		VMInfo:    vmInfo,
		UIButtons: buttons,
	}

	c.JSON(&me, w, r)
}
