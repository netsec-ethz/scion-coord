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
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

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

type passwordRequest struct {
	UUID                 string `json:"uuid"`
	Password             string `json:"password"`
	PasswordConfirmation string `json:"password_confirmation"`
}

type verificationResponse struct {
	Activated bool
	FirstName string
	LastName  string
}

var notifyAdmin bool = true

// check if the password match and that the length is at least 8 chars
func passwordsAreValid(password, passwordConfirmation string) error {
	if len(password) < 8 {
		return fmt.Errorf("%s\n", "Please use at least 8 characters for your password.")
	}

	if password != passwordConfirmation {
		return fmt.Errorf("%s\n", "Please enter matching passwords.")
	}

	return nil
}

// Method used to validate the registration request
func (r *registrationRequest) isValid() error {

	//check recaptcha
	rc := recaptcha.R{Secret: config.CAPTCHA_SECRET_KEY}
	if !rc.VerifyResponse(r.Captcha) {
		return fmt.Errorf("ReCaptcha error: %s", strings.Join(rc.LastError()[1:], ", "))
	}

	// check if any of this is empty
	if r.Email == "" || r.Password == "" || r.PasswordConfirmation == "" ||
		r.First == "" || r.Last == "" {
		return fmt.Errorf("%s\n", "You entered incomplete data. First and last name, email and "+
			"password are mandatory fields.")
	}

	// check if the password match and that the length is at least 8 chars
	return passwordsAreValid(r.Password, r.PasswordConfirmation)
}

// Method used to reset password and send user an email
func (c *RegistrationController) ResetPassword(w http.ResponseWriter, r *http.Request) {

	// parse the form value
	if err := r.ParseForm(); err != nil {
		log.Println(err)
		c.Error500(w, err, "Parsing form values failed.")
		return
	}

	userEmail := r.FormValue("userEmail")
	u, err := models.FindUserByEmail(userEmail)
	if err != nil {
		log.Println(err)
		c.BadRequest(w, err, "User not found")
		return
	}

	if err = u.ResetUUID(); err != nil {
		log.Printf("Error resetting UUID for user %v: %v", u.Email, err)
		c.BadRequest(w, err, "Error resetting UUID")
		return
	}
	if err = u.UpdatePassword(""); err != nil {
		log.Printf("Error resetting password for user %v: %v", u.Email, err)
		c.BadRequest(w, err, "Error resetting password")
		return
	}
	data := email.EmailData{
		FirstName:        u.FirstName,
		LastName:         u.LastName,
		HostAddress:      config.HTTP_HOST_ADDRESS,
		VerificationUUID: u.VerificationUUID,
		Protocol:         config.HTTP_PROTOCOL,
	}
	if err = email.ConstructAndSend(
		"password_reset.html",
		"[SCIONLab] Password reset",
		data,
		"password-reset",
		userEmail); err != nil {
		log.Printf("Error sending password-reset email to user %v: %v", u.Email, err)
		c.BadRequest(w, err, "Error sending email")
		return
	}

	return
}

// Method used to set password after pre-approved registration or password reset
func (c *RegistrationController) SetPassword(w http.ResponseWriter, r *http.Request) {

	// parse the form value
	if err := r.ParseForm(); err != nil {
		log.Println(err)
		c.Error500(w, err, "Parsing form values failed")
		return
	}

	// parse the JSON coming from the client
	var passRequest passwordRequest
	decoder := json.NewDecoder(r.Body)

	// check if the parsing succeeded
	if err := decoder.Decode(&passRequest); err != nil {
		log.Println(err)
		c.Error500(w, err, "Error parsing form values failed")
		return
	}

	if err := passwordsAreValid(passRequest.Password, passRequest.PasswordConfirmation); err != nil {
		log.Println(err)
		c.Error500(w, err, "Password invalid")
		return
	}

	//validate link
	user, err := models.FindUserByVerificationUUID(passRequest.UUID)

	if err != nil {
		log.Printf("Error setting password: %v is not a valid UUID", passRequest.UUID)
		c.BadRequest(w, nil, "Error verifying email address: %v is not a valid user identifier", passRequest.UUID)
		return
	}

	if !user.PasswordInvalid {
		c.Error500(w, nil, "Password is already set")
		return
	}

	if err := user.UpdatePassword(passRequest.Password); err != nil {
		log.Printf("Error updating the password in the database: %v", err)
		c.Error500(w, err, "Error updating the password in the database")
		return
	}

	c.Plain("", w, r)
	return
}

// VerifyEmail verifies a user's email address and sets the user to activated if trusted
func (c *RegistrationController) VerifyEmail(w http.ResponseWriter, r *http.Request) {

	// retrieve submitted uuid
	uuid := r.PostFormValue("uuid")

	// validate link
	u, err := models.FindUserByVerificationUUID(uuid)
	if err != nil {
		log.Printf("Error verifying email address: %v is not a valid UUID.", uuid)
		c.BadRequest(w, nil, "Error verifying email address: %v is not a valid user identifier", uuid)
		return
	}

	// check if user is already verified
	if u.Verified {
		log.Printf("User %v is already verified.", u.Email)
	} else {
		// update user
		if err := u.UpdateVerified(true); err != nil {
			log.Printf("Error verifying email address for user %v: %v.", u.Email, err)
			c.Error500(w, nil, "Error verifying email address for user %v", u.Email)
			return
		}
	}

	// if user activation feature is turned on and the user is trusted set it to activated
	// if the feature is off, also set it to activated
	if trusted, err := trustedDomain(u.Email); err != nil { // error checking against trusted domains
		log.Printf("Error checking %v against trusted domains: %v", u.Email, err)
		c.Error500(w, err, "Error checking %v against trusted domains", u.Email)
		return
	} else if !config.USER_ACTIVATION || trusted { // email domain is whitelisted or activation feature is turned off
		if err := u.UpdateActivated(true); err != nil {
			log.Printf("Error activating user %v: %v", u.Email, err)
			c.Error500(w, err, "Error activating user %v", u.Email)
			return
		}
	} else if notifyAdmin { // email domain is not whitelisted and feature is turned on, needs manual activation
		// notify admins
		admins, err := loadAdmins()
		if err != nil {
			log.Printf("Error sending notification to administrators: %v", err)
		} else {
			data := email.EmailData{
				Protocol:    config.HTTP_PROTOCOL,
				HostAddress: config.HTTP_HOST_ADDRESS,
			}

			email.ConstructAndSend(
				"pending_activations.html",
				"[SCIONLab] Pending user activations",
				data,
				"scion-pending",
				admins...)

			// disable admin notifications for subsequent users
			notifyAdmin = false
		}
	}

	data := verificationResponse{
		Activated: u.Activated,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}

	c.JSON(&data, w, r)
}

// This method is used to register a new account via the standard form
func (c *RegistrationController) Register(w http.ResponseWriter, r *http.Request) {

	// parse the form value
	if err := r.ParseForm(); err != nil {
		log.Println(err)
		c.Error500(w, err, "Error parsing form values")
		return
	}

	// parse the JSON coming from the client
	var regRequest registrationRequest
	decoder := json.NewDecoder(r.Body)

	// check if the parsing succeeded
	if err := decoder.Decode(&regRequest); err != nil {
		log.Println(err)
		c.Error500(w, err, "Error decoding JSON")
		return
	}

	// validate the data
	if err := regRequest.isValid(); err != nil {
		log.Println(err)
		c.Error500(w, err, "Invalid form data")
		return
	}

	// register the user
	account := regRequest.Email // use the user's email as a unique account
	user, err := models.RegisterUser(account, regRequest.Organisation,
		regRequest.Email, regRequest.Password, regRequest.First, regRequest.Last)

	if err != nil {
		log.Printf("Error registering the user: %v", err)
		c.Error500(w, err, "Error registering the user")
		return
	} else {
		c.JSON(&user, w, r)
	}

	// Send email address confirmation link
	if err := sendVerificationEmail(user.Id); err != nil {
		log.Printf("Error sending verification email: %v", err)
		c.Error500(w, err, "Error sending verification email")
	}

}

func (c *RegistrationController) LoadCaptchaSiteKey(w http.ResponseWriter, r *http.Request) {
	c.Plain(config.CAPTCHA_SITE_KEY, w, r)
}

func (c *RegistrationController) ResendActivationLink(w http.ResponseWriter, r *http.Request) {

	user, err := models.FindUserByEmail(r.PostFormValue("email"))
	if err != nil {
		c.Error500(w, err, "User %v was not found", r.PostFormValue("email"))
		return
	}

	if user.Verified {
		c.Error500(w, nil, "User %v is already verified", user.Email)
		return
	}

	if err := sendVerificationEmail(user.Id); err != nil {
		log.Printf("Error sending verification email: %v", err)
		c.Error500(w, err, "Error sending verification email")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
	w.WriteHeader(http.StatusNoContent)
}

// Function which sends verification emails to newly registered users
func sendVerificationEmail(userID uint64) error {

	user, err := models.FindUserById(fmt.Sprintf("%v", userID))
	if err != nil {
		return err
	}

	data := email.EmailData{
		FirstName:        user.FirstName,
		LastName:         user.LastName,
		Protocol:         config.HTTP_PROTOCOL,
		HostAddress:      config.HTTP_HOST_ADDRESS,
		VerificationUUID: user.VerificationUUID,
	}

	if err := email.ConstructAndSend(
		"verification.html",
		"[SCIONLab] Verify your email address for SCIONLab Coordination Service",
		data,
		"email-verification",
		user.Email); err != nil {
		return err
	}

	return nil
}

// Helper function that checks if a given email address is from a trusted domain
func trustedDomain(email string) (bool, error) {
	file, _ := os.Open("conf/trusted_domains")
	defer file.Close()
	scanner := bufio.NewScanner(file)
	b := false
	for scanner.Scan() {
		ref := scanner.Text()
		if strings.HasSuffix(email, "."+ref) || strings.HasSuffix(email, "@"+ref) || email == ref {
			b = true
			break
		}
	}

	return b, scanner.Err()
}

func loadAdmins() ([]string, error) {
	var ids []string
	u, err := models.FindUsersByRole(true)
	if err != nil {
		log.Printf("Loading admins failed: %v", err)
		return nil, err
	}
	for _, user := range *u {
		ids = append(ids, user.Email)
	}
	return ids, nil
}
