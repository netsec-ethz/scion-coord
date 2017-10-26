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

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/api"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion/go/lib/addr"
)

// make sure that data about SCIONLab ASes in database is correct
// TODO (mlegner): remove deprecated servers?
// TODO (mlegner): replace this by an init script and an admin interface at some point.
func initializeSLS() error {
	for _, s := range config.SERVERS {
		if err := models.InsertUpdateSLS(s); err != nil {
			return err
		}
	}

	return nil
}

// check if credential files exist and create necessary directories
func checkCredentialsDirectories() error {
	for _, server := range config.SERVERS {
		ia, err := addr.IAFromString(server.IA)
		if err != nil {
			return err
		}
		for _, f := range []string{api.TrcFile(ia.I), api.CoreCertFile(ia.I),
			api.CoreSigKey(ia.I)} {
			if _, err := os.Stat(f); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("ERROR: Credential file %s does not exist. Please make "+
						"sure that the necessary credential files exist.\n"+
						"Consult the README.md for further details.", f)
				} else {
					return fmt.Errorf("An error occurred when accessing " + f + ".")
				}
			}
		}
	}
	os.MkdirAll(api.TempPath, os.ModePerm)
	os.MkdirAll(api.PackagePath, os.ModePerm)
	return nil
}

func main() {

	// update database of SCIONLab ASes
	if err := checkCredentialsDirectories(); err != nil {
		fmt.Printf("There was an error checking credential files: %v", err)
		return
	}

	// check if credential files exist and create necessary directories
	if err := initializeSLS(); err != nil {
		fmt.Printf("There was an error updating the server database: %v", err)
		return
	}

	// controllers
	registrationController := api.RegistrationController{}
	loginController := api.LoginController{}
	adminController := api.AdminController{}
	asController := api.ASController{}
	scionLabVMController := api.SCIONLabVMController{}

	// rate limitation
	resendLimit := tollbooth.NewLimiter(1, time.Minute*10,
		&limiter.ExpirableOptions{DefaultExpirationTTL: time.Hour})
	resendLimit.SetOnLimitReached(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Blocked %v from accessing '/api/resendLink' because of reached rate limit",
			w.Header().Get("X-Rate-Limit-Request-Remote-Addr"))
	})
	resendLimit.SetMessage("You can request an email every 10 minutes")

	// router
	router := mux.NewRouter()

	loggingChain := middleware.NewWithLogging()

	// public chain does not require authentication but serves back the XSRF Token
	xsrfChain := middleware.NewWithLogging(middleware.XSRFHandler)

	// Api chain goes through the authentication handler, which verifies either the session or the
	// account_id.secret combination
	apiChain := middleware.NewWithLogging(middleware.AuthHandler)

	// User chain goes through UserHandler which checks if the user is logged in
	userChain := middleware.NewWithLogging(middleware.UserHandler)

	// Admin chain goes through AdminHandler which checks if user is marked as admin
	adminChain := middleware.NewWithLogging(middleware.AdminHandler)

	// handle favicon requests
	router.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "public/img/favicon.ico")
	}))

	// index page
	router.Handle("/", xsrfChain.ThenFunc(controllers.Index))

	// ==========================================================
	// SCION Coord API

	// user registration
	router.Handle("/api/register", loggingChain.ThenFunc(
		registrationController.Register)).Methods(http.MethodPost)
	router.Handle("/api/captchaSiteKey", loggingChain.ThenFunc(
		registrationController.LoadCaptchaSiteKey))

	// Resend verification email
	router.Handle("/api/resendLink", tollbooth.LimitHandler(resendLimit, loggingChain.ThenFunc(
		registrationController.ResendActivationLink))).Methods(http.MethodPost)

	// user login
	router.Handle("/api/login", loggingChain.ThenFunc(loginController.Login))

	// user Logout
	router.Handle("/api/logout", loggingChain.ThenFunc(loginController.Logout))

	// user information
	router.Handle("/api/userPageData", apiChain.ThenFunc(loginController.UserInformation))

	// email validation
	router.Handle("/api/verifyEmail/{uuid}", loggingChain.ThenFunc(
		registrationController.VerifyEmail))

	// set password after pre-approved registration or password reset
	router.Handle("/api/setPassword", loggingChain.ThenFunc(
		registrationController.SetPassword)).Methods(http.MethodPost)

	// admin page
	router.Handle("/api/adminPageData", adminChain.ThenFunc(adminController.AdminInformation))
	router.Handle("/api/sendInvitations", adminChain.ThenFunc(
		adminController.SendInvitationEmails)).Methods(http.MethodPost)

	// generates a SCIONLab VM
	// TODO(ercanucan): fix the authentication
	router.Handle("/api/as/generateVM", userChain.ThenFunc(
		scionLabVMController.GenerateSCIONLabVM)).Methods(http.MethodPost)
	router.Handle("/api/as/removeVM", userChain.ThenFunc(scionLabVMController.RemoveSCIONLabVM))
	router.Handle("/api/as/downloads", userChain.ThenFunc(scionLabVMController.ReturnTarball))
	router.Handle("/api/as/getSCIONLabVMASes/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabVMController.GetSCIONLabVMASes))
	router.Handle("/api/as/confirmSCIONLabVMASes/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabVMController.ConfirmSCIONLabVMASes))

	// ==========================================================
	// SCION Web API

	router.Handle("/api/as/exists/{as_id}/{account_id}/{secret}", apiChain.ThenFunc(
		asController.Exists))

	// ISD join request
	router.Handle("/api/as/uploadJoinRequest/{account_id}/{secret}", apiChain.ThenFunc(
		asController.UploadJoinRequest))
	router.Handle("/api/as/uploadJoinReply/{account_id}/{secret}", apiChain.ThenFunc(
		asController.UploadJoinReply))
	router.Handle("/api/as/pollJoinReply/{account_id}/{secret}", apiChain.ThenFunc(
		asController.PollJoinReply))

	// AS connection request
	router.Handle("/api/as/uploadConnRequest/{account_id}/{secret}", apiChain.ThenFunc(
		asController.UploadConnRequest))
	router.Handle("/api/as/uploadConnReply/{account_id}/{secret}", apiChain.ThenFunc(
		asController.UploadConnReply))

	// show all request TO this AS
	router.Handle("/api/as/pollEvents/{account_id}/{secret}", apiChain.ThenFunc(
		asController.PollEvents))

	// list the ASes the requesting AS can connect to
	router.Handle("/api/as/listASes/{account_id}/{secret}", apiChain.ThenFunc(
		asController.ListASes))

	// serve static files
	static := http.StripPrefix("/public/", http.FileServer(http.Dir("public")))
	router.PathPrefix("/public/").Handler(xsrfChain.Then(static))

	// listen to HTTP requests
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.HTTP_BIND_ADDRESS,
		config.HTTP_BIND_PORT), handlers.CompressHandler(router)))
}
