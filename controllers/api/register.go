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
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/haisum/recaptcha"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/email"
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
	Captcha              string `json:"captcha"`
}

// Method used to validate the registration request
func (r *registrationRequest) isValid() (bool, error) {

	//check recaptcha
	rc := recaptcha.R{Secret: config.CAPTCHA_SECRET_KEY}
	if !rc.VerifyResponse(r.Captcha) {
		return false, fmt.Errorf("ReCaptcha error: %s", strings.Join(rc.LastError()[1:], ", "))
	}

	// check if any of this is empty
	if r.Email == "" || r.Password == "" || r.PasswordConfirmation == "" ||
		r.First == "" || r.Last == "" {
		return false, fmt.Errorf("%s\n", "You entered incomplete data. First and last name, email and password are mandatory fields.")
	}

	// check if the password match and that the length is at least 8 chars
	if len(r.Password) < 8 {

		return false, fmt.Errorf("%s\n", "Please use at least 8 characters for your password.")
	}

	if r.Password != r.PasswordConfirmation {
		return false, fmt.Errorf("%s\n", "Please enter matching passwords.")
	}

	return true, nil
}

// Method used to validate email address
func (c *RegistrationController) VerifyEmail(w http.ResponseWriter, r *http.Request) {

	//retrieve submitted uuid
	uuid := mux.Vars(r)["uuid"]

	//validate link
	u, err := models.FindUserByVerificationUUID(uuid)

	if err != nil {
		log.Printf("Error verifying email address. %v is not a valid UUID.", uuid)
		c.BadRequest(fmt.Errorf("Error verifying email address. %v is not a valid user identifier.", uuid), w, r)
		return
	}

	if u.Verified {
		log.Printf("User %v is already verified.", u.Email)
	} else {
		// update user
		if err := u.UpdateVerified(true); err != nil {
			log.Printf("Error verifying email address for user %v: %v.", u.Email, err)
			// TODO: Pass the user a unique error ID which links to the specific error and allows for debugging
			c.Error500(fmt.Errorf("Error verifying email address for user %v.", u.Email), w, r)
			return
		}
	}

	// load validation page
	t, err := template.ParseFiles("templates/layout.html", "templates/verified.html")
	if err != nil {
		log.Printf("Error parsing HTML files: %v", err)
		c.Error500(err, w, r)
		return
	}
	c.Render(t, u, w, r)

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
	account := regRequest.Email // use the user's email as a unique account
	user, err := models.RegisterUser(account, regRequest.Organisation,
		regRequest.Email, regRequest.Password, regRequest.First, regRequest.Last)

	if err != nil {
		log.Printf("Error registering the user: %v", err)
		c.Error500(err, w, r)
		return
	} else {
		c.JSON(&user, w, r)
	}

	// Send email address confirmation link
	if err := sendMail(user.Id); err != nil {
		log.Printf("Error sending verification email: %v", err)
		c.Error500(err, w, r)
	}

}

func (c *RegistrationController) LoadCaptchaSiteKey(w http.ResponseWriter, r *http.Request) {
	c.Plain(config.CAPTCHA_SITE_KEY, w, r)
}

func (c *RegistrationController) ResendActivationLink(w http.ResponseWriter, r *http.Request) {

	user, err := models.FindUserByEmail(r.PostFormValue("email"))
	if err != nil {
		c.Error500(err, w, r)
		return
	}

	if user.Verified {
		c.Error500(fmt.Errorf("User %v is already verified", user.Email), w, r)
		return
	}

	if err := sendMail(user.Id); err != nil {
		log.Printf("Error sending verification email: %v", err)
		c.Error500(err, w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusNoContent)
}

// Helper function which creates the email and server objects used to send emails to users
func sendMail(userID uint64) error {

	user, err := models.FindUserById(fmt.Sprintf("%v", userID))
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFiles("email/templates/verification.html")
	if err != nil {
		return err
	}

	data := struct {
		FirstName        string
		LastName         string
		HostAddress      string
		VerificationUUID string
	}{user.FirstName, user.LastName, config.HTTP_HOST_ADDRESS, user.VerificationUUID}

	buf := new(bytes.Buffer)
	tmpl.Execute(buf, data)
	body := buf.String()

	mail := new(email.Email)
	mail.From = config.EMAIL_FROM
	mail.To = []string{user.Email}
	mail.Subject = "[SCIONLab] Verify your email address for SCIONLab Coordination Service"
	mail.Body = body

	if err := email.Send(mail); err != nil {
		return err
	}

	return nil
}
