package api

import (
	"encoding/json"
	"errors"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
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
	c.Render(t, nil, w, r)
}

// Method used to validate the registration request
func (r *registrationRequest) isValid() bool {
	// check if any of this is empty
	if r.Email == "" || r.Organisation == "" || r.Password == "" || r.PasswordConfirmation == "" ||
		r.First == "" || r.Last == "" || r.Account == "" {
		return false
	}

	// check if the password match and that the length is at least 8 chars
	if len(r.Password) < 8 || r.Password != r.PasswordConfirmation {

		return false
	}

	return true
}

// This method is used to register a new account via the standard form
func (c *RegistrationController) RegisterPost(w http.ResponseWriter, r *http.Request) {

	_, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		c.Error500(err, w, r)
		return
	}

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
		c.Error500(err, w, r)
		return
	}

	// validate the data
	if !regRequest.isValid() {
		c.Error500(errors.New("{}"), w, r)
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
		c.Error500(err, w, r)
		return
	}

	// validate the data
	if !regRequest.isValid() {
		c.Error500(errors.New("{}"), w, r)
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
}
