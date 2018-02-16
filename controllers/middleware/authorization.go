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
	"net/http"

	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/models"
)

type CheckFunction func(r *http.Request) bool

var (
	AuthHandler  = constructHandler(checkAPI)
	UserHandler  = constructHandler(checkLogin)
	AdminHandler = constructHandler(checkAdmin)
)

// TODO(mlegner): We need an additional authorization handler that checks if the account is an admin
func checkAccountSecret(r *http.Request) bool {
	vars := mux.Vars(r)
	accountID := vars["account_id"]
	secret := vars["secret"]
	// In this case we are receiving a request with account_id and secret params
	if accountID != "" && secret != "" {
		if account, err := models.FindAccountByAccountIDAndSecret(accountID, secret); err == nil &&
			account != nil {
			// proceed with the next handler
			return true
		}

	}

	// try with standard Golang parameters
	accountID = r.URL.Query().Get("account_id")
	secret = r.URL.Query().Get("secret")

	if accountID != "" && secret != "" {
		if account, err := models.FindAccountByAccountIDAndSecret(accountID, secret); err == nil &&
			account != nil {
			// proceed with the next handler
			return true
		}
	}
	return false
}

func checkLogin(r *http.Request) bool {
	_, userSession, err := GetUserSession(r)
	if err == nil && userSession != nil && userSession.HasLoggedIn {
		return true
	}
	return false
}

func checkAdmin(r *http.Request) bool {
	_, userSession, _ := GetUserSession(r)
	if userSession != nil && userSession.HasLoggedIn && userSession.IsAdmin {
		return true
	}
	return false
}

func checkAPI(r *http.Request) bool {
	return checkAccountSecret(r) || checkLogin(r)
}

func constructHandler(checkFunc CheckFunction) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if checkFunc(r) {
				// proceed with the next handler
				next.ServeHTTP(w, r)
				return
			} else {
				http.Error(w, "Not authorized", http.StatusForbidden)
				return
			}

		})
	}
}
