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
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/njern/gogmail"
	"html/template"
	"log"
	"net/http"
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
func (c *RegistrationController) Validate(w http.ResponseWriter, r *http.Request) {
	

	//load validation page
	t, err := template.ParseFiles("templates/validate.html")
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	c.Render(t, nil, w, r)

	//print params

	 email := r.URL.Query().Get("email")
	 fmt.Println("-------------------")
	 fmt.Println("Email address is:", r.URL.Query().Get("email"))
 	 fmt.Println("Hash is:", r.URL.Query().Get("hash"))
	 fmt.Println("-------------------")

	 if email == "claude.haehni@ethz.ch" {
		fmt.Println("in");
		//update user
		user, _ := models.FindUserByEmail(email)
		user.UpdateVerified(true)
	 }

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
	if user, err := models.RegisterUser(regRequest.Account, regRequest.Organisation,
		regRequest.Email, regRequest.Password, regRequest.First, regRequest.Last); err != nil {
		c.Error500(errors.New("{}"), w, r)
		return
	} else {
		c.JSON(&user, w, r)
	}

	//Send email address confirmation link
	gmail := gogmail.GmailConnection("sendingfromgo@gmail.com", "simplepassword")
	if err := gmail.SendMail([]string{regRequest.Email}, "Confirm your email address", "http://127.0.0.1:8080/api/validateEmail/" + regRequest.Email + "/" ); err != nil {
		fmt.Println(err);
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
	if user, err := models.RegisterUser(regRequest.Account, regRequest.Organisation,
		regRequest.Email, regRequest.Password, regRequest.First, regRequest.Last); err != nil {
		log.Println(err)
		c.Error500(err, w, r)
		return
	} else {
		c.JSON(&user, w, r)
	}

	//Send email address confirmation link
	gmail := gogmail.GmailConnection("sendingfromgo@gmail.com", "simplepassword")
	if err := gmail.SendMail([]string{regRequest.Email}, "Confirm your email address", "http://127.0.0.1:8080/api/validateEmail/" + regRequest.Email + "/" ); err != nil {
		fmt.Println(err);
	}
}
