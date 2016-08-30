package main

import (
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/api"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"log"
	"net/http"
)

func main() {

	// controllers
	registrationController := api.RegistrationController{}
	loginController := api.LoginController{}
	asController := api.ASController{}
	adminController := api.AdminController{}

	// router
	router := mux.NewRouter()

	loggingChain := middleware.New(middleware.LoggingHandler)

	// public chain does not require authentication but serves back the XSRF Token
	xsrfChain := middleware.New(middleware.LoggingHandler, middleware.XSRFHandler)

	// Api chain goes through the authentication handler, which verifies either the session or the key.secret
	// combination
	apiChain := middleware.New(middleware.LoggingHandler, middleware.AuthHandler)

	// 404 on favicon requests
	router.Handle("/favicon.ico", http.HandlerFunc(http.NotFound))

	// index page for registration and login
	router.Handle("/", xsrfChain.ThenFunc(controllers.Index))

	// login page
	router.Handle("/login", xsrfChain.ThenFunc(loginController.LoginPage))

	// register page
	router.Handle("/register", xsrfChain.ThenFunc(registrationController.RegisterPage))
	router.Handle("/register", xsrfChain.ThenFunc(registrationController.RegisterPost)).Methods("POST")

	// admin page
	router.Handle("/admin", loggingChain.ThenFunc(adminController.Index))

	// registration
	router.Handle("/api/register", loggingChain.ThenFunc(registrationController.Register))
	// login
	router.Handle("/api/login", loggingChain.ThenFunc(loginController.Login))
	// Logout
	router.Handle("/api/logout", loggingChain.ThenFunc(loginController.Logout))
	// Me

	router.Handle("/api/me", loggingChain.ThenFunc(loginController.Me))

	// ==========================================================
	// API
	router.Handle("/api/as/exists/{as_id}/{key}/{secret}", apiChain.ThenFunc(asController.Exists))
	router.Handle("/api/as/insert/{key}/{secret}", apiChain.ThenFunc(asController.Insert))

	router.Handle("/api/as/uploadJoinRequest/{key}/{secret}", apiChain.ThenFunc(asController.UploadJoinRequest))
	router.Handle("/api/as/pollJoinReply/{key}/{secret}", apiChain.ThenFunc(asController.PollJoinReply))
	router.Handle("/api/as/uploadConnRequests/{key}/{secret}", apiChain.ThenFunc(asController.UploadConnRequests))
	router.Handle("/api/as/pollConnReplies/{key}/{secret}", apiChain.ThenFunc(asController.PollConnReplies))
	router.Handle("/api/as/uploadJoinReplies/{key}/{secret}", apiChain.ThenFunc(asController.UploadJoinReplies))
	router.Handle("/api/as/uploadConnReplies/{key}/{secret}", apiChain.ThenFunc(asController.UploadConnReplies))
	router.Handle("/api/as/pollEvents/{key}/{secret}", apiChain.ThenFunc(asController.PollEvents))

	// serve static files
	static := http.StripPrefix("/public/", http.FileServer(http.Dir("public")))
	router.PathPrefix("/public/").Handler(xsrfChain.Then(static))

	// listen to HTTP requests
	log.Fatal(http.ListenAndServe(config.HTTP_HOST+":"+config.HTTP_PORT, handlers.CompressHandler(router)))
}
