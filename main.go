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

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/api"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
)

func main() {
	// check if credential files exist and create necessary directories
	for _, f := range []string{api.TrcFile, api.CoreCertFile, api.CoreSigKey} {
		if _, err := os.Stat(f); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("ERROR: Please make sure that the necessary credential files exist.")
				fmt.Println("Consult the README.md for further details.")
			} else {
				fmt.Println("An error occurred when accessing " + f + ".")
			}
			return
		}
	}
	os.MkdirAll(api.TempPath, os.ModePerm)
	os.MkdirAll(api.PackagePath, os.ModePerm)

	// controllers
	registrationController := api.RegistrationController{}
	loginController := api.LoginController{}
	asController := api.ASController{}
	scionLabVMController := api.SCIONLabVMController{}

	// router
	router := mux.NewRouter()

	loggingChain := middleware.New(middleware.LoggingHandler)

	// public chain does not require authentication but serves back the XSRF Token
	xsrfChain := middleware.New(middleware.LoggingHandler, middleware.XSRFHandler)

	// Api chain goes through the authentication handler, which verifies either the session or the account_id.secret
	// combination
	apiChain := middleware.New(middleware.LoggingHandler, middleware.AuthHandler)

	// 404 on favicon requests
	router.Handle("/favicon.ico", http.HandlerFunc(http.NotFound))

	// index page
	router.Handle("/", xsrfChain.ThenFunc(controllers.Index))

	// ==========================================================
	// SCION Coord API

	// user registration
	router.Handle("/api/register", loggingChain.ThenFunc(registrationController.Register)).Methods("POST")
	router.Handle("/api/captchaSiteKey", loggingChain.ThenFunc(registrationController.LoadCaptchaSiteKey))
	// Resend verification email
	router.Handle("/api/resendLink", loggingChain.ThenFunc(registrationController.ResendActivationLink))
	// user login
	router.Handle("/api/login", loggingChain.ThenFunc(loginController.Login))
	// user Logout
	router.Handle("/api/logout", loggingChain.ThenFunc(loginController.Logout))
	// user information
	router.Handle("/api/me", loggingChain.ThenFunc(loginController.Me))

	//email validation
	router.Handle("/api/verifyEmail/{uuid}", loggingChain.ThenFunc(registrationController.VerifyEmail))

	// generates a SCIONLab VM
	// TODO(ercanucan): fix the authentication
	router.Handle("/api/as/generateVM", apiChain.ThenFunc(scionLabVMController.GenerateSCIONLabVM))
	router.Handle("/api/as/removeVM", apiChain.ThenFunc(scionLabVMController.RemoveSCIONLabVM))
	router.Handle("/api/as/downloads", apiChain.ThenFunc(scionLabVMController.ReturnTarball))
	router.Handle("/api/as/getSCIONLabVMASes/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabVMController.GetSCIONLabVMASes))
	router.Handle("/api/as/confirmSCIONLabVMASes/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabVMController.ConfirmSCIONLabVMASes))

	// ==========================================================
	// SCION Web API

	router.Handle("/api/as/exists/{as_id}/{account_id}/{secret}", apiChain.ThenFunc(asController.Exists))

	// ISD join request
	router.Handle("/api/as/uploadJoinRequest/{account_id}/{secret}", apiChain.ThenFunc(asController.UploadJoinRequest))
	router.Handle("/api/as/uploadJoinReply/{account_id}/{secret}", apiChain.ThenFunc(asController.UploadJoinReply))
	router.Handle("/api/as/pollJoinReply/{account_id}/{secret}", apiChain.ThenFunc(asController.PollJoinReply))

	// AS connection request
	router.Handle("/api/as/uploadConnRequest/{account_id}/{secret}", apiChain.ThenFunc(asController.UploadConnRequest))
	router.Handle("/api/as/uploadConnReply/{account_id}/{secret}", apiChain.ThenFunc(asController.UploadConnReply))

	// show all request TO this AS
	router.Handle("/api/as/pollEvents/{account_id}/{secret}", apiChain.ThenFunc(asController.PollEvents))

	// list the ASes the requesting AS can connect to
	router.Handle("/api/as/listASes/{account_id}/{secret}", apiChain.ThenFunc(asController.ListASes))

	// serve static files
	static := http.StripPrefix("/public/", http.FileServer(http.Dir("public")))
	router.PathPrefix("/public/").Handler(xsrfChain.Then(static))

	// listen to HTTP requests
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", config.HTTP_BIND_ADDRESS, config.HTTP_BIND_PORT), handlers.CompressHandler(router)))
}
