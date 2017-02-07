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
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"html/template"
	"log"
	"net/http"
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
	Account      string
	Organisation string
	AccountId    string
	Secret       string
}

// TODO: cache the templates
func (c *LoginController) LoginPage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/layout.html", "templates/login.html")
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	c.Render(t, nil, w, r)
}

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
		AccountId:    storedUser.Account.AccountId,
		Secret:       storedUser.Account.Secret,
	}

	c.JSON(&user, w, r)

}

func (c *LoginController) Logout(w http.ResponseWriter, r *http.Request) {
	// get the current user session if present.
	// if not then, abort
	session, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		log.Println(err)
		c.Forbidden(err, w, r)
		return
	}

	// expire the session
	session.Options.MaxAge = -1

	if err := session.Save(r, w); err != nil {
		c.Error500(err, w, r)
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
		c.Forbidden(err, w, r)
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
			c.Forbidden(err, w, r)
			return
		}

		// assign the decoded values
		email = user.Email
		password = user.Password

		// make sure they are not empty
		if email == "" || password == "" {
			c.Forbidden(err, w, r)
			return
		}

	}

	// load the user and verify email and password authentication
	// if succeeded then, set the information in the user session
	// otherwise redirect to the home page
	dbUser, err := models.FindUserByEmail(email)
	if err != nil || dbUser == nil {
		c.BadRequest(err, w, r)
		return
	}

	// if the authentication fails
	if err := dbUser.Authenticate(password); err != nil {
		log.Println(err)
		c.Forbidden(err, w, r)
		return
	}

	// otherwise just continue, because the authentication succeeded
	// TODO: rotate the session
	userSession.Email = dbUser.Email
	userSession.HasLoggedIn = true
	userSession.First = dbUser.FirstName
	userSession.Last = dbUser.LastName
	userSession.Organisation = dbUser.Account.Organisation

	// fill in the properties of the struct to return to the front end app
	user.FirstName = dbUser.FirstName
	user.LastName = dbUser.LastName
	user.Account = dbUser.Account.Name
	user.Organisation = dbUser.Account.Organisation

	// clean up the password
	user.Password = ""

	// set the session value
	session.Values[middleware.ScionSessionName] = userSession

	// save the session status
	if err := session.Save(r, w); err != nil {
		log.Println("Error while saving the session", err)
		c.Error500(err, w, r)
		return
	}

	// if the user session is valid and the user is logged in, then continue, otherwise redirect to the home page
	if userSession != nil && userSession.HasLoggedIn {
		// the session is valid, therefore continue
		c.JSON(&user, w, r)

	} else {
		log.Println("AUth error")
		c.Forbidden(err, w, r)
		return
	}

}
