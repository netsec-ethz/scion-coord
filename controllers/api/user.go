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

type asInfo struct {
	ASStatus uint8
	ASText   string
	ASIP     string
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
	Update   buttonConfiguration // Button to create or update AS
	Download buttonConfiguration // Button to re-download AS
	Remove   buttonConfiguration // Button to remove AS
}

type userPageData struct {
	User      user
	ASInfo    asInfo
	UIButtons uiButtons
}

// generates the structs containing information about the user's AS and the
// configuration of UI buttons
func populateASStatusButtons(userEmail string) (asInfo, uiButtons, error) {
	asInfo := asInfo{}
	buttons := uiButtons{
		Update: buttonConfiguration{
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
		Remove: buttonConfiguration{
			Text:            "Remove my SCIONLab AS Configuration",
			Class:           "btn-danger",
			Action:          "remove",
			TooltipDisabled: "You currently do not have an active AS configuration.",
			Disable:         true,
		},
	}

	as, err := models.FindOneSCIONLabASByUserEmail(userEmail)
	if err != nil {
		if err != orm.ErrNoRows {
			return asInfo, buttons, err
		}
	} else {
		asInfo.ASIP = as.PublicIP
		asInfo.ASStatus = as.Status
	}
	switch asInfo.ASStatus {
	case models.INACTIVE:
		asInfo.ASText = "You currently do not have an active SCIONLab AS."
		buttons.Update.Text = "Create and Download SCIONLab AS Configuration"
		buttons.Update.Disable = false
		buttons.Download.Hide = true
		buttons.Remove.Hide = true
	case models.ACTIVE:
		asInfo.ASText = "You currently have an active SCIONLab AS."
		buttons.Update.Disable = false
		buttons.Remove.Disable = false
	case models.CREATE:
		asInfo.ASText = "You have a pending creation request for your SCIONLab AS."
	case models.UPDATE:
		asInfo.ASText = "You have a pending update request for your SCIONLab AS."
	case models.REMOVE:
		asInfo.ASText = "Your SCIONLab AS configuration is currently scheduled for removal."
		buttons.Download.Disable = true
	}
	if asInfo.ASStatus == models.ACTIVE || asInfo.ASStatus == models.CREATE ||
		asInfo.ASStatus == models.UPDATE {
		// TODO(mlegner): This is only a temporary fix until multiple connections are implemented
		if as.PublicIP == "" {
			asInfo.ShowVPN = true
		} else {
			asInfo.ShowIP = true
		}
	}

	return asInfo, buttons, nil
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

	asInfo, buttons, err := populateASStatusButtons(user.Email)
	if err != nil {
		log.Printf("Error when generating AS info and button configuration for user %v: %v",
			user.Email, err)
		c.Forbidden(w, err, "Error when generating AS info and button configuration")
		return
	}

	userData := userPageData{
		User:      user,
		ASInfo:    asInfo,
		UIButtons: buttons,
	}

	c.JSON(&userData, w, r)
}
