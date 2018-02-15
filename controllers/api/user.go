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

	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
)

type asInfo struct {
	ASID    int       // AS ID of the user's SCIONLab AS
	ISD     int       // Current ISD of the user's SCIONLab AS
	Label   string    // Label of the AS
	IALabel string    // ISD-AS string + Label of the AS
	Status  uint8     // Current status of the AS
	IP      string    // IP address of the AS
	Type    uint8     // Type of the SCIONLab AS
	IsVPN   bool      // Is this a VPN-based setup
	AP      string    // ISD-AS of the connected Attachment Point
	Port    uint16    // Port of BR on the user's AS
	ASText  string    // Text to be displayed by the frontend
	Buttons uiButtons // Buttons shown for this AS
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
	Configure  buttonConfiguration // Button to create or update AS
	Download   buttonConfiguration // Button to re-download AS
	Disconnect buttonConfiguration // Button to remove AS
}

type userPageData struct {
	User    user
	MaxASes int // maximal number of ASes this user can have
	APs     []string
	ASInfos []asInfo
}

// generates the structs containing information about the user's AS and the
// configuration of UI buttons
func populateASStatusButtons(userEmail string) ([]asInfo, []string, error) {
	asInfos := []asInfo{}
	apInfos := []string{}
	ases, err := models.FindSCIONLabASesByUserEmail(userEmail)
	if err != nil {
		return asInfos, apInfos, err
	}
	aps, err := models.GetAllAPs()
	if err != nil {
		return asInfos, apInfos, err
	}
	for _, ap := range aps {
		// TODO(mlegner): Add label
		apInfos = append(apInfos, ap.IA())
	}
	for _, as := range ases {
		buttons := uiButtons{
			Configure: buttonConfiguration{
				Text:            "Update and Download SCIONLab AS Configuration",
				Action:          "update",
				TooltipDisabled: "Updates are disabled as you have a pending request.",
				Disable:         true,
			},
			Download: buttonConfiguration{
				Text:            "Re-download my SCIONLab AS Configuration",
				Action:          "download",
				TooltipDisabled: "You currently do not have an active AS configuration.",
			},
			Disconnect: buttonConfiguration{
				Text:            "Disconnect my SCIONLab AS from the Network",
				Class:           "btn-danger",
				Action:          "remove",
				TooltipDisabled: "You currently do not have an active AS configuration.",
				Disable:         true,
			},
		}

		asI := asInfo{
			ASID:    as.ASID,
			ISD:     as.ISD,
			IALabel: as.String(),
			Label:   as.Label,
			Status:  as.Status,
			IP:      as.PublicIP,
			Type:    as.Type,
			Port:    as.StartPort,
		}

		cns, err := as.GetJoinConnectionInfo()
		if err != nil {
			return asInfos, apInfos, err
		}
		// TODO(mlegner): Currently only one connection allowed
		if len(cns) > 0 {
			asI.IsVPN = cns[0].IsVPN
			asI.AP = utility.IAString(cns[0].NeighborISD, cns[0].NeighborAS)
		}

		switch asI.Status {
		case models.INACTIVE:
			asI.ASText = "This AS is currently inactive."
			buttons.Configure.Text = "Create and Download SCIONLab AS Configuration"
			buttons.Configure.Disable = false
			buttons.Download.Hide = true
			buttons.Disconnect.Hide = true
		case models.ACTIVE:
			asI.ASText = "This AS is currently active."
			buttons.Configure.Disable = false
			buttons.Disconnect.Disable = false
		case models.CREATE:
			asI.ASText = "You have a pending creation request for your SCIONLab AS."
		case models.UPDATE:
			asI.ASText = "You have a pending update request for your SCIONLab AS."
		case models.REMOVE:
			asI.ASText = "Your SCIONLab AS configuration is currently scheduled for removal."
			buttons.Download.Disable = true
		}
		if asI.Type == models.INFRASTRUCTURE {
			buttons.Configure.Hide = true
			buttons.Download.Hide = true
			buttons.Disconnect.Hide = true
		}
		asI.Buttons = buttons

		asInfos = append(asInfos, asI)
	}

	return asInfos, apInfos, nil
}

// generates the user-information struct to be used in dynamic HTML pages
func populateUserData(r *http.Request) (u user, err error) {
	// get the current user session if present.
	// if not then, abort
	_, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
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
		AccountID:    storedUser.Account.AccountID,
		Secret:       storedUser.Account.Secret,
	}

	return
}

// API function that generates all information necessary for displaying the user page
func (c *LoginController) UserInformation(w http.ResponseWriter, r *http.Request) {

	user, err := populateUserData(r)
	if err != nil {
		log.Println(err)
		c.Forbidden(w, err, "Error authenticating user: Not logged in")
		return
	}

	asInfo, aps, err := populateASStatusButtons(user.Email)
	if err != nil {
		log.Printf("Error when generating AS info and button configuration for user %v: %v",
			user.Email, err)
		c.Forbidden(w, err, "Error when generating AS info and button configuration")
		return
	}

	userData := userPageData{
		User:    user,
		MaxASes: config.MaxASes(user.IsAdmin),
		ASInfos: asInfo,
		APs:     aps,
	}

	c.JSON(&userData, w, r)
}
