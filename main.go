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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/limiter"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/api"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/scionproto/scion/go/lib/addr"
	"golang.org/x/crypto/acme/autocert"
)

// initialize ISD location mapping
func initializeISD() error {
	raw, err := ioutil.ReadFile(config.ISDLocationMapping)
	if err != nil {
		return fmt.Errorf("ERROR: Cannot access ISD location mapping json file: %v", err)
	}
	var isdLocs []struct {
		ISD       addr.ISD
		Country   string
		Continent string
	}
	json.Unmarshal(raw, &isdLocs)

	for _, ISD := range isdLocs {
		_, err = models.FindISDbyID(ISD.ISD)
		if err == orm.ErrNoRows {
			isd := models.ISDLocation{
				ISD:       ISD.ISD,
				Country:   ISD.Country,
				Continent: ISD.Continent,
			}
			err = isd.Insert()
		}
		if err != nil {
			return fmt.Errorf("ERROR: Cannot insert ISD location mapping into database:"+
				" %v", err)
		}
	}
	return nil
}

// check if credential files exist and create necessary directories
func checkCredentialsDirectories() error {
	aps, err := models.GetAllAPs()
	if err != nil {
		return err
	}
	for _, ap := range aps {
		isd := ap.ISD
		for _, f := range []string{api.TrcFile(isd), api.CoreCertFile(isd),
			api.CoreSigKey(isd)} {
			if _, err := os.Stat(f); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("ERROR: Credential file %s does not exist. Please make "+
						"sure that the necessary credential files exist.\n"+
						"Consult the README.md for further details.", f)
				}
				return fmt.Errorf("An error occurred when accessing " + f + ".")
			}
		}
	}
	os.MkdirAll(api.TempPath, os.ModePerm)
	os.MkdirAll(api.PackagePath, os.ModePerm)
	return nil
}

func main() {
	if err := initializeISD(); err != nil {
		fmt.Printf("There was an error updating"+
			" the ISD location mapping in the database: %v", err)
		return
	}

	// check if credential files exist and create necessary directories
	if err := checkCredentialsDirectories(); err != nil {
		fmt.Printf("There was an error checking credential files: %v", err)
		return
	}

	// controllers
	registrationController := api.RegistrationController{}
	loginController := api.LoginController{}
	userController := api.UserController{}
	adminController := api.AdminController{}
	asController := api.ASInfoController{}
	scionLabASController := api.SCIONLabASController{}
	scionBoxController := api.SCIONBoxController{}
	scionImageBuildController := api.CreateSCIONImgBuildController()

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

	// resend verification email
	router.Handle("/api/resendLink", tollbooth.LimitHandler(resendLimit, loggingChain.ThenFunc(
		registrationController.ResendActivationLink))).Methods(http.MethodPost)

	// reset user password
	router.Handle("/api/resetPassword", tollbooth.LimitHandler(resendLimit, loggingChain.ThenFunc(
		registrationController.ResetPassword))).Methods(http.MethodPost)

	// user login
	router.Handle("/api/login", loggingChain.ThenFunc(loginController.Login))

	// user Logout
	router.Handle("/api/logout", loggingChain.ThenFunc(loginController.Logout))

	// user information
	router.Handle("/api/userPageData", apiChain.ThenFunc(userController.UserInformation))

	// change password by logged-in users
	router.Handle("/api/changePassword", userChain.ThenFunc(
		userController.ChangePassword)).Methods(http.MethodPost)

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

	// generates a SCIONLab AS
	// TODO(ercanucan): fix the authentication
	router.Handle("/api/as/generateAS", userChain.ThenFunc(
		scionLabASController.GenerateNewSCIONLabAS)).Methods(http.MethodPost)
	router.Handle("/api/as/configureAS", userChain.ThenFunc(
		scionLabASController.ConfigureSCIONLabAS)).Methods(http.MethodPost)
	router.Handle("/api/as/removeAS/{as_id}", userChain.ThenFunc(
		scionLabASController.RemoveSCIONLabAS))
	router.Handle("/api/as/downloadTarball/{as_id}", userChain.ThenFunc(
		scionLabASController.ReturnTarball))
	router.Handle("/api/as/remapId/{as_id}", loggingChain.ThenFunc(
		scionLabASController.RemapASIdentityChallengeAndSolution)).Methods(http.MethodGet, http.MethodPost)
	router.Handle("/api/as/remapIdDownloadGen/{as_id}", loggingChain.ThenFunc(
		scionLabASController.RemapASDownloadGen)).Methods(http.MethodPost)
	router.Handle("/api/as/remapIdConfirmStatus/{as_id}", loggingChain.ThenFunc(
		scionLabASController.RemapASConfirmStatus)).Methods(http.MethodPost)
	router.Handle("/api/as/getUpdatesForAP/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabASController.GetUpdatesForAP))
	router.Handle("/api/as/confirmUpdatesFromAP/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabASController.ConfirmUpdatesFromAP))
	router.Handle("/api/as/queryUpdateBranch/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabASController.QueryUpdateBranch))
	router.Handle("/api/as/confirmUpdate/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabASController.ConfirmUpdate)).Methods(http.MethodPost)
	// full synchronization (not only pending changes) for the APs:
	router.Handle("/api/as/getConnectionsForAP/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabASController.GetConnectionsForAP))
	router.Handle("/api/as/setConnectionsForAP/{account_id}/{secret}",
		apiChain.ThenFunc(scionLabASController.SetConnectionsForAP))

	//SCIONBox API
	router.Handle("/api/as/initBox", loggingChain.ThenFunc(scionBoxController.InitializeBox))
	router.Handle("/api/as/connectBox/{account_id}/{secret}", apiChain.ThenFunc(
		scionBoxController.ConnectNewBox))
	router.Handle("/api/as/heartbeat/{account_id}/{secret}", apiChain.ThenFunc(
		scionBoxController.HeartBeatFunction))

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

	// ==========================================================
	// Virtual currency API

	router.Handle("/api/listASConnections/{account_id}/{secret}/{ia}",
		apiChain.ThenFunc(asController.ListASesConnectionsWithCredits))

	// ==========================================================
	// Image building API

	router.Handle("/api/imgbuild/images",
		apiChain.ThenFunc(scionImageBuildController.GetAvailableDevices))

	router.Handle("/api/imgbuild/create/{as_id}",
		apiChain.ThenFunc(scionImageBuildController.GenerateImage)).Methods(http.MethodPost)

	router.Handle("/api/imgbuild/user-images",
		apiChain.ThenFunc(scionImageBuildController.GetUserImages))

	// serve static files
	static := http.StripPrefix("/public/", http.FileServer(http.Dir("public")))
	router.PathPrefix("/public/").Handler(xsrfChain.Then(static))

	// serve website using https or standard http
	if config.HTTPEnableHTTPS {
		fmt.Printf("Serving website on %v over HTTPS\n", config.HTTPHostAddress)
		// redirect HTTP traffic to HTTPS
		go http.ListenAndServe(":80", http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
			}))

		// listen to HTTPS requests
		log.Fatal(http.Serve(autocert.NewListener(config.HTTPHostAddress), router))
	} else {
		fmt.Printf("Serving website on %v over HTTP\n", config.HTTPHostAddress)
		log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d",
			config.HTTPBindAddress, config.HTTPBindPort), router))
	}

}
