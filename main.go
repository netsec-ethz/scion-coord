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

	// admin page
	router.Handle("/admin", xsrfChain.ThenFunc(adminController.Index))

	// regitration
	router.Handle("/api/register", xsrfChain.ThenFunc(registrationController.Register))
	// login
	router.Handle("/api/login", xsrfChain.ThenFunc(loginController.Login))
	// Logout
	router.Handle("/api/logout", xsrfChain.ThenFunc(loginController.Logout))
	// Me
	router.Handle("/api/me", xsrfChain.ThenFunc(loginController.Me))

	// ==========================================================
	// API
	router.Handle("/api/as/exists/{as_id}/{key}/{secret}", apiChain.ThenFunc(asController.Exists))
	router.Handle("/api/as/upsert/{key}/{secret}", apiChain.ThenFunc(asController.Upsert))

	// serve static files
	static := http.StripPrefix("/public/", http.FileServer(http.Dir("public")))
	router.PathPrefix("/public/").Handler(xsrfChain.Then(static))

	// listen to HTTP requests
	log.Fatal(http.ListenAndServe(config.HTTP_HOST+":"+config.HTTP_PORT, handlers.CompressHandler(router)))
}
