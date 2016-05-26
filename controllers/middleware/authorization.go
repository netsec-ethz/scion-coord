package middleware

import (
	//"log"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/models"
	"net/http"
)

// TODO: distinguish between web interface user authentication and key/secret authentication
// The latter does not need a session
func AuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// auth the user via the session
		_, userSession, _ := GetUserSession(r)
		if userSession != nil && userSession.HasLoggedIn {
			// proceed with the next handler
			next.ServeHTTP(w, r)
			return
		}

		// try then with the key and secret
		vars := mux.Vars(r)
		key := vars["key"]
		secret := vars["secret"]
		// In this case we are receiving a request with key and secret params
		if key != "" && secret != "" {
			if account, err := models.FindUserByKeySecret(key, secret); err == nil && account != nil {
				// proceed with the next handler
				next.ServeHTTP(w, r)
				return
			}

		}

		// try with standard Golang parameters
		key = r.URL.Query().Get("key")
		secret = r.URL.Query().Get("secret")

		if key != "" && secret != "" {
			if account, err := models.FindUserByKeySecret(key, secret); err == nil && account != nil {
				// proceed with the next handler
				next.ServeHTTP(w, r)
				return
			}
		}

		// if we got here means that the XSRF token was not valid
		http.Error(w, "Not authorized", http.StatusForbidden)
		return

	})
}
