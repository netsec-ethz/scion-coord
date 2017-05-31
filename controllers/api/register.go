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
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
)

type RegistrationController struct {
	controllers.HTTPController
}

type registrationRequest struct {
	Email                string `json:"email"`
	Organisation         string `json:"organisation"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
	First                string `json:"first"`
	Last                 string `json:"last"`
	Account              string `json:"account"`
}

// TODO: cache the templates
func (c *RegistrationController) RegisterPage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/layout.html", "templates/register.html")
	if err != nil {
		c.Error500(err, w, r)
		return
	}

	_, userSession, err := middleware.GetUserSession(r)
	if err != nil || userSession == nil {
		c.Error500(err, w, r)
		return
	}

	c.Render(t, userSession, w, r)
}

// Method used to validate the registration request
func (r *registrationRequest) isValid() (bool, error) {
	// check if any of this is empty
	if r.Email == "" || r.Organisation == "" || r.Password == "" || r.PasswordConfirmation == "" ||
		r.First == "" || r.Last == "" || r.Account == "" {
		return false, fmt.Errorf("%s\n", "Email, Organisation, Password, Password confirmation, Firts and Last name and Account are all mandatory fields")
	}

	// check if the password match and that the length is at least 8 chars
	if len(r.Password) < 8 {

		return false, fmt.Errorf("%s\n", "Password length invalid. Use at leats 8 chars")
	}

	if r.Password != r.PasswordConfirmation {
		return false, fmt.Errorf("%s\n", "Password mismatch")
	}

	return true, nil
}

// Method used to validate email address
func (c *RegistrationController) Verify(w http.ResponseWriter, r *http.Request) {

	//retrieve submitted uuid
	link := mux.Vars(r)["link"]

	//validate link
	u, err := models.FindUserByEmailLink(link)

	if err != nil {
		c.BadRequest(errors.New("invalid link"), w, r)
		return
	}

	if u.Verified {
		c.BadRequest(errors.New("user alreday verified"), w, r)
		return
	}

	//update user
	u.UpdateVerified(true)

	//load validation page
	t, err := template.ParseFiles("templates/layout.html", "templates/verified.html")
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	c.Render(t, nil, w, r)

}

// This method is used to register a new account via the standard form
func (c *RegistrationController) RegisterPost(w http.ResponseWriter, r *http.Request) {

	session, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		c.Error500(err, w, r)
		return
	}

	// parse the form value
	if err := r.ParseForm(); err != nil {
		log.Println(err)
		userSession.Error = "Could not parse the input data. Try again."
		session.Save(r, w)
		c.Redirect(302, "/register", w, r)
		return
	}

	// parse the JSON coming from the client
	var regRequest registrationRequest
	decoder := json.NewDecoder(r.Body)

	// check if the parsing succeeded
	if err := decoder.Decode(&regRequest); err != nil {
		userSession.Error = "Could not parse the input data. Try again."
		session.Save(r, w)
		c.Redirect(302, "/register", w, r)
		return
	}

	// validate the data
	if valid, err := regRequest.isValid(); !valid {
		userSession.Error = err.Error()
		session.Save(r, w)
		c.Redirect(302, "/register", w, r)
		return
	}

	// register the user
	user, err := models.RegisterUser(regRequest.Account, regRequest.Organisation,
		regRequest.Email, regRequest.Password, regRequest.First, regRequest.Last)

	if err != nil {
		c.Error500(errors.New("{}"), w, r)
		return
	} else {
		c.JSON(&user, w, r)
	}

	//Send email address confirmation link
	link := user.EmailLink

	mail := new(models.EMail)
	mail.From = "test@example.com"
	mail.To = []string{regRequest.Email}
	mail.Subject = "Verify email address"
	mail.Body = "http://" + config.HTTP_HOST + ":" + config.HTTP_PORT + "/api/verify/" + link

	server := &email.SMTPServer{Host: "127.0.0.1", Port: 25}

	if err := email.Send(mail, server); err != nil {
		c.Error500(err, w, r)
	}

}

// This method is used to register a new account via the standard form
func (c *RegistrationController) Register(w http.ResponseWriter, r *http.Request) {

	// parse the form value
	if err := r.ParseForm(); err != nil {
		log.Println(err)
		http.Error(w, "{}", http.StatusInternalServerError)
		return
	}

	// parse the JSON coming from the client
	var regRequest registrationRequest
	decoder := json.NewDecoder(r.Body)

	// check if the parsing succeeded
	if err := decoder.Decode(&regRequest); err != nil {
		log.Println(err)
		c.Error500(err, w, r)
		return
	}

	// validate the data
	if valid, err := regRequest.isValid(); !valid {
		log.Println(err)
		c.Error500(err, w, r)
		return
	}

	// register the user
	user, err := models.RegisterUser(regRequest.Account, regRequest.Organisation,
		regRequest.Email, regRequest.Password, regRequest.First, regRequest.Last)

	if err != nil {
		c.Error500(errors.New("{}"), w, r)
		return
	} else {
		c.JSON(&user, w, r)
	}

	//Send email address confirmation link
	link := user.EmailLink

	mail := new(models.EMail)
	mail.From = "test@example.com"
	mail.To = []string{regRequest.Email}
	mail.Subject = "Verify email address"
	mail.Body = "http://" + config.HTTP_HOST + ":" + config.HTTP_PORT + "/api/verify/" + link

	server := &email.SMTPServer{Host: "127.0.0.1", Port: 25}

	if err := email.Send(mail, server); err != nil {
		c.Error500(err, w, r)
	}
}
