// Copyright 2016 ETH Zurich
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
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
)

const (
	emailFieldName    = "email"
	passwordFieldName = "password"
)

type LoginController struct {
	controllers.HTTPController
}

type user struct {
	Email        string
	Password     string
	FirstName    string
	LastName     string
	IsAdmin      bool
	Account      string
	Organisation string
	AccountID    string
	Secret       string
}

func (c *LoginController) Logout(w http.ResponseWriter, r *http.Request) {
	// get the current user session if present.
	// if not then, abort
	session, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		log.Println(err)
		c.Forbidden(w, err, "Error getting user session")
		return
	}

	// expire the session
	session.Options.MaxAge = -1

	if err := session.Save(r, w); err != nil {
		c.Error500(w, err, "Error: Session expired")
		return
	}

}

// This method is used to validate username and password
func (c *LoginController) Login(w http.ResponseWriter, r *http.Request) {

	// get the current user session if present.
	// if not then, abort
	session, userSession, err := middleware.GetUserSession(r)

	if err != nil {
		log.Println(err)
		c.Forbidden(w, err, "Error getting user session")
		return
	}

	// User session was found, so try to authenticate
	var user user

	// we have already parsed the query string in the previous handler XSRF
	email := r.FormValue(emailFieldName)
	password := r.FormValue(passwordFieldName)
	if email == "" || password == "" {
		// if the form fields are empty, then try by parsing a json payload

		// parse the JSON coming from the client
		decoder := json.NewDecoder(r.Body)

		// check if the parsing succeeded
		if err := decoder.Decode(&user); err != nil {
			c.Forbidden(w, err, "Error decoding JSON")
			return
		}

		// assign the decoded values
		email = user.Email
		password = user.Password

		// make sure they are not empty
		if email == "" || password == "" {
			c.Forbidden(w, nil, "Email or password empty")
			return
		}

	}

	// load the user and verify email and password authentication
	// if succeeded then, set the information in the user session
	// otherwise redirect to the home page
	dbUser, err := models.FindUserByEmail(email)
	if err != nil || dbUser == nil {
		c.BadRequest(w, err, "Error: User not found")
		return
	}

	// if the authentication fails
	if err := dbUser.Authenticate(password); err != nil {
		log.Printf("Authentication failed for user %v: %v", dbUser.Email, err)
		// Distinguish between different authentication errors
		// The web interface uses this information to react accordingly
		// 900: Email is not verified, 901: wrong password, 902: Password is invalid (reset), 903: User is not activated
		if err.Error() == "Email is not verified" {
			c.Forbidden(w, err, "900 Authentication failed for user %v", dbUser.Email)
			return
		} else if err.Error() == "Password is invalid" {
			c.Forbidden(w, err, "902 Authentication failed for user %v", dbUser.Email)
		} else if err.Error() == "User is not activated" {
			c.Forbidden(w, err, "903 Authentication failed for user %v", dbUser.Email)
		} else {
			c.Forbidden(w, err, "901 Authentication failed for user %v", dbUser.Email)
			return
		}
	}

	// otherwise just continue, because the authentication succeeded
	// TODO: rotate the session
	userSession.Email = dbUser.Email
	userSession.HasLoggedIn = true
	userSession.IsAdmin = dbUser.IsAdmin
	userSession.First = dbUser.FirstName
	userSession.Last = dbUser.LastName
	userSession.Organisation = dbUser.Account.Organisation

	// fill in the properties of the struct to return to the front end app
	user.FirstName = dbUser.FirstName
	user.LastName = dbUser.LastName
	user.IsAdmin = dbUser.IsAdmin
	user.Account = dbUser.Account.Name
	user.Organisation = dbUser.Account.Organisation

	// clean up the password
	user.Password = ""

	// set the session value
	session.Values[middleware.ScionSessionName] = userSession

	// save the session status
	if err := session.Save(r, w); err != nil {
		log.Printf("Error while saving the session: %v", err)
		c.Error500(w, err, "Error while saving the session")
		return
	}

	// if the user session is valid and the user is logged in, then continue, otherwise redirect to the home page
	if userSession != nil && userSession.HasLoggedIn {
		// the session is valid, therefore continue
		c.JSON(&user, w, r)
	} else {
		log.Println("Authentication error")
		c.Forbidden(w, nil, "Authentication error")
		return
	}

}
