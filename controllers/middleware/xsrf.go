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
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/models"
)

var (
	store = sessions.NewFilesystemStore(config.SESSION_PATH,
		[]byte(config.SESSION_VERIFICATION_KEY), []byte(config.SESSION_ENCRYPTION_KEY)) // random value to validate the session
	sessionName              = "session"                                            // cookie name
	ScionSessionName         = "scion-session"                                      // key name in the session map
	tokenSize                = 16                                                   // size of the token
	xsrfTokenGenerationError = errors.New("Could not generate a valid XSRF token.") // error in case the RAND function fails
	emptyArray               = make([]byte, tokenSize)                              // empty array to double check the result of the rand function
	formXSRFParameter        = "xsrf"                                               // the expected name of the XSRF parameter
	xsrfHeaderName           = "X-Xsrf-Token"
	//sessionCache             = make(map[string]*models.Session)                     // caches the user session in memory, for later use in different handlers. It's filled in only here
	//sessionMutex             = sync.Mutex{}
)

func XSRFHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Get a session. If a session does not exist, a new one is generated.
		// If a client session exists, but the server does not know about it, then an error is raised
		// In this last case we generate a new session.
		session, err := store.Get(r, sessionName)

		// if it's a new session then generate a new scion-coord session, with an XSRF Token
		if session != nil && session.IsNew || err != nil {
			// in this case the token was not present, because it's a new session
			token, err := generateXSRFToken()

			// in case of error generating the token, return - logging is provided by the generateXSRFToken function
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			// Create a new session object
			newSession := models.Session{
				XSRFToken: token,
			}

			// set the session value
			session.Values[ScionSessionName] = newSession
			// save the session status
			if err := session.Save(r, w); err != nil {
				log.Println("Error while saving the session", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			// set the xsrf token as http header.
			// Angular is already configured to use this.
			w.Header().Set(xsrfHeaderName, token)

			// proceed with the next handler
			next.ServeHTTP(w, r)

			// stop here and move on to the next handler
			return

		}

		// if the user session was already present
		// then check whether the user sends an HTTP header with the right xsrf token
		// or a form parameter
		if userSession, present := session.Values[ScionSessionName]; present {

			// handle deserialization problems of the user session
			if sess, ok := userSession.(*models.Session); !ok {
				// Handle the case that it's not an expected type
				// if we got here means that the XSRF token was not valid
				http.Error(w, "XSRF Token mismatch", http.StatusForbidden)
				return
			} else {

				// get the session token
				token := sess.XSRFToken

				// add the xsrf token header
				w.Header().Set(xsrfHeaderName, token)

				// validate the xsrf token
				if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
					// check Angular JS default HTTP headers
					angularToken := r.Header.Get(xsrfHeaderName)
					// means we can try to validate angular passed token
					if angularToken != "" && subtle.ConstantTimeCompare([]byte(angularToken), []byte(token)) == 1 {
						next.ServeHTTP(w, r)
						return
					}

					// parse the values coming from the request
					if err := r.ParseForm(); err != nil {
						log.Println("XSRF handler, error parsing form values", err)
					}

					if subtle.ConstantTimeCompare([]byte(r.FormValue(formXSRFParameter)), []byte(token)) == 1 {
						next.ServeHTTP(w, r)
						return
					}

					// if we got here means that the XSRF token was not valid
					http.Error(w, "XSRF Token mismatch", http.StatusForbidden)
					return
				}

			}
		}

		// if we got here means that it was not a POST/PUT/DELETE request
		// proceed with the next handler
		next.ServeHTTP(w, r)

	})
}

func generateXSRFToken() (string, error) {
	buffer := make([]byte, tokenSize)
	_, err := rand.Read(buffer)
	if err != nil {
		log.Println("XSRF Token generation error:", err)
		return "", err
	}
	// The slice should now contain random bytes instead of only zeroes.
	if bytes.Equal(buffer, emptyArray) {
		// then we have a problem!
		return "", xsrfTokenGenerationError
	}

	return hex.EncodeToString(buffer), nil
}

// if the returned session is nil, it means the session did not exist
// if the return session is not nil, then session existed before
// if the function returns an error, then we need to stop because the session could not be
// deserialized
func GetUserSession(r *http.Request) (*sessions.Session, *models.Session, error) {

	requestSession, err := store.Get(r, sessionName)
	if err != nil {
		return nil, nil, errors.New("Could not get or generate a new session. Most likely a session configration problem. Check the session path if it exists.")
	}

	// this should always be true, but just in case double check it
	if userSession, present := requestSession.Values[ScionSessionName]; present {
		if sess, ok := userSession.(*models.Session); !ok {
			// Handle the case that it's not an expected type
			return nil, nil, errors.New("Could not deserialize SCION session")
		} else {
			// otherwise return the user session
			return requestSession, sess, nil
		}

	}

	return nil, nil, errors.New("SCION session not present")
}
