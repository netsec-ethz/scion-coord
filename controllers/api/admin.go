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
	"io/ioutil"
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/email"
	"github.com/netsec-ethz/scion-coord/models"
)

type AdminController struct {
	controllers.HTTPController
}

type adminPageData struct {
	User           user
	EmailMessage   string
	UserActivation bool
}

type invitationInfo struct {
	FirstName    string
	LastName     string
	Email        string
	Organisation string
}

type emailTemplateInfo struct {
	FirstName        string
	LastName         string
	InviterFirstName string
	InviterLastName  string
	Protocol         string
	HostAddress      string
	UUID             string
}

type invitationsData []invitationInfo

var invitationsTemplate = "invitation.html"

func (c AdminController) AdminInformation(w http.ResponseWriter, r *http.Request) {
	user, err := populateUserData(r)
	if err != nil {
		log.Printf("Error authenticating user: %v", err)
		c.Forbidden(w, err, "Error authenticating user")
		return
	}

	// TODO (mlegner): Fill in template except FirstName and LastName
	text, err := ioutil.ReadFile(email.EmailTemplatePath(invitationsTemplate))

	adminData := adminPageData{
		User:           user,
		EmailMessage:   string(text),
		UserActivation: config.USER_ACTIVATION,
	}

	c.JSON(&adminData, w, r)
	return
}

func preregisterAndSendInvitation(userSession *models.Session, invitation *invitationInfo) error {
	// register the user without password
	account := invitation.Email // use the user's email as a unique account
	user, err := models.RegisterUser(account, invitation.Organisation,
		invitation.Email,
		"", invitation.FirstName, invitation.LastName)
	if err != nil {
		return err
	}

	err = user.UpdateVerified(true)
	if err != nil {
		return err
	}

	data := emailTemplateInfo{
		FirstName:        invitation.FirstName,
		LastName:         invitation.LastName,
		InviterFirstName: userSession.First,
		InviterLastName:  userSession.Last,
		Protocol:         config.HTTP_PROTOCOL,
		HostAddress:      config.HTTP_HOST_ADDRESS,
		UUID:             user.VerificationUUID,
	}

	email.ConstructAndSend(
		"invitation.html",
		"[SCIONLab] Invitation to join the SCION network",
		data,
		"scion-invitation",
		invitation.Email)

	return nil
}

func (c AdminController) SendInvitationEmails(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		log.Printf("There was an error parsing the form for email invitations: %v", err)
		c.BadRequest(w, err, "There was an error parsing form for email invitations")
		return
	}

	// parse the JSON coming from the client
	decoder := json.NewDecoder(r.Body)
	var invitations invitationsData

	// check if the parsing succeeded
	if err := decoder.Decode(&invitations); err != nil {
		log.Printf("Error decoding json data for email invitations: %v", err)
		c.Error500(w, err, "Error decoding json data for email invitations")
		return
	}

	session, userSession, err := middleware.GetUserSession(r)
	if session == nil || err != nil {
		log.Printf("No user session found: %v", err)
		c.Forbidden(w, err, "No user session found")
	}

	errorEmails := []string{}
	errors := []string{}
	for _, invitation := range invitations {
		err := preregisterAndSendInvitation(userSession, &invitation)
		if err != nil {
			log.Printf("Error sending invitation email to %v: %v", invitation.Email, err)
			errorEmails = append(errorEmails, invitation.Email)
			errors = append(errors, controllers.Verbosity(err, "Could not send email to user %v", invitation.Email))
		} else {
			errors = append(errors, "")
		}
	}

	if len(errors) == 0 {
		c.Plain("", w, r)
		return
	} else {
		c.JSON(map[string][]string{"messages": errors, "emails": errorEmails}, w, r)
		return
	}
}

// LoadUnactivatedUsers loads all users from db that are verified but not yet activated and passes it to the front end
func (c AdminController) LoadUnactivatedUsers(w http.ResponseWriter, r *http.Request) {

	if !config.USER_ACTIVATION {
		log.Printf("Error loading inactive users: User activation feature is turned off")
		c.Error500(w, nil, "Error loading inactive users: User activation feature is turned off")
		return
	}

	// load the users from the database
	users, err := models.GetVerifiedUnactivatedUsers()
	if err != nil {
		log.Printf("Error loading inactive users: %v", err)
		c.Error500(w, err, "Error loading inactive users")
		return
	}

	// strip away unneeded data
	// NOTE: due to a limitation with the database driver this can not be done via the database query yet
	for i := range *users {
		account := new(models.Account)
		account.Organisation = (*users)[i].Account.Organisation
		(*users)[i].Account = account
	}

	// admin will be notified again once a new user registers
	notifyAdmin = true

	c.JSON(users, w, r)
}

func (c AdminController) ActivateUser(w http.ResponseWriter, r *http.Request) {

	if !config.USER_ACTIVATION {
		log.Printf("Error loading inactive users: User activation feature is turned off")
		c.Error500(w, nil, "Error loading inactive users: User activation feature is turned off")
		return
	}

	// read email from request form
	userEmail := r.PostFormValue("email")
	if userEmail == "" {
		log.Printf("Error activating user: email is empty")
		c.Error500(w, nil, "Error activating user: email is empty")
		return
	}

	// find user by email and activate
	user, err := models.FindUserByEmail(userEmail)
	if err != nil {
		log.Printf("Error: User with email %v not found: %v", userEmail, err)
		c.Error500(w, err, "User with email %v not found", userEmail)
		return
	}

	if err := user.UpdateActivated(true); err != nil {
		log.Printf("Error activating user %v: %v", userEmail, err)
		c.Error500(w, err, "Error activating user %v", userEmail)
		return
	}

	// send notification email to user
	data := email.EmailData{
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Protocol:    config.HTTP_PROTOCOL,
		HostAddress: config.HTTP_HOST_ADDRESS,
	}

	email.ConstructAndSend(
		"activation.html",
		"[SCIONLab] Activation of your account",
		data,
		"scion-activation",
		user.Email)

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusNoContent)

}
