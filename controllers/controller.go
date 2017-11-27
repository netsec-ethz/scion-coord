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

// This is the main interface file which defines the methods of the controller
package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-coord/config"
)

type output interface {
	// returns JSON data
	JSON(data interface{}, w http.ResponseWriter, r *http.Request)
	// returns plain text data
	Plain(data string, w http.ResponseWriter, r *http.Request)
	// Response with a 500 and an error
	Error500(w http.ResponseWriter, err error, desc string, a ...interface{})

	BadRequest(w http.ResponseWriter, err error, desc string, a ...interface{})

	NotFound(w http.ResponseWriter, err error, desc string, a ...interface{})

	Forbidden(w http.ResponseWriter, err error, desc string, a ...interface{})

	Render(tpl *template.Template, data interface{}, w http.ResponseWriter, r *http.Request)

	Redirect(code int, url string, w http.ResponseWriter, r *http.Request)
}

type HTTPController struct {
	output
}

// Verbosity returns an error string containing sensitive information when
// debug mode is activated or a more generic error message otherwise
var Verbosity func(err error, format string, a ...interface{}) string

func init() {
	if config.LOG_DEBUG_MODE {
		Verbosity = func(err error, format string, a ...interface{}) string {
			if err != nil {
				return fmt.Sprintf(format+": %v", append(a, err)...)
			}
			return fmt.Sprintf(format, a...)
		}
	} else {
		Verbosity = func(err error, format string, a ...interface{}) string {
			return fmt.Sprintf(format, a...)
		}
	}
}

func (c HTTPController) JSON(data interface{}, w http.ResponseWriter, r *http.Request) {

	var content []byte
	var err error

	// set the JSON header
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	//marshall the data into json bytes
	content, err = json.Marshal(data)

	// in case of marshalling error
	// return 500
	if err != nil {
		c.Error500(w, err, "Error creating response")
		return
	}

	// write the content to socket
	if _, err := w.Write(content); err != nil {
		log.Printf("Error writing data to socket: %v", err)
		c.Error500(w, err, "Error creating response")
	}

}

func (c HTTPController) Plain(data string, w http.ResponseWriter, r *http.Request) {

	// Set plain text header
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// write the content to socket
	if _, err := w.Write([]byte(data)); err != nil {
		log.Printf("Error writing data to socket: %v", err)
		c.Error500(w, err, "Error creating response")
	}
}

func (c HTTPController) Error(w http.ResponseWriter, err error, errorCode int, description string, a ...interface{}) {

	// Log the error
	log.Println(fmt.Sprintf(description+": %v", append(a, err)...))

	// Forward the error to the web interface
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, Verbosity(err, description, a...), errorCode)

}

func (c HTTPController) Error500(w http.ResponseWriter, err error, desc string, a ...interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, Verbosity(err, desc, a...), http.StatusInternalServerError)
}

func (c HTTPController) BadRequest(w http.ResponseWriter, err error, desc string, a ...interface{}) {

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, Verbosity(err, desc, a...), http.StatusBadRequest)
}

func (c HTTPController) NotFound(w http.ResponseWriter, err error, desc string, a ...interface{}) {

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, Verbosity(err, desc, a...), http.StatusNotFound)
}

func (c HTTPController) Forbidden(w http.ResponseWriter, err error, desc string, a ...interface{}) {

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, Verbosity(err, desc, a...), http.StatusForbidden)
}

func (C HTTPController) Render(tpl *template.Template, data interface{}, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	tpl.Execute(w, data)
}

func (C HTTPController) Redirect(code int, url string, w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, url, code)
}
