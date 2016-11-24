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

package middleware

import (
	//"log"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/models"
	"net/http"
)

// TODO: distinguish between web interface user authentication and account_id/secret authentication
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

		// try then with the account_id and secret
		vars := mux.Vars(r)
		account_id := vars["account_id"]
		secret := vars["secret"]
		// In this case we are receiving a request with account_id and secret params
		if account_id != "" && secret != "" {
			if account, err := models.FindUserByAccountIdSecret(account_id, secret); err == nil && account != nil {
				// proceed with the next handler
				next.ServeHTTP(w, r)
				return
			}

		}

		// try with standard Golang parameters
		account_id = r.URL.Query().Get("account_id")
		secret = r.URL.Query().Get("secret")

		if account_id != "" && secret != "" {
			if account, err := models.FindUserByAccountIdSecret(account_id, secret); err == nil && account != nil {
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
