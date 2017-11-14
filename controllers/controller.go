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
	"html/template"
	"log"
	"net/http"
)

type output interface {
	// returns JSON data
	JSON(data interface{}, w http.ResponseWriter, r *http.Request)
	// returns plain text data
	Plain(data string, w http.ResponseWriter, r *http.Request)
	// Response with a 500 and an error
	Error500(err error, w http.ResponseWriter, r *http.Request)

	BadRequest(err error, w http.ResponseWriter, r *http.Request)

	NotFound(err error, w http.ResponseWriter, r *http.Request)

	Forbidden(err error, w http.ResponseWriter, r *http.Request)

	Render(tpl *template.Template, data interface{}, w http.ResponseWriter, r *http.Request)

	Redirect(code int, url string, w http.ResponseWriter, r *http.Request)
}

type HTTPController struct {
	output
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
		c.Error500(err, w, r)
		return
	}

	// write the content to socket
	if _, err := w.Write(content); err != nil {
		log.Printf("Error writing data to socket: %v", err)
		c.Error500(err, w, r)
	}

}

func (c HTTPController) Plain(data string, w http.ResponseWriter, r *http.Request) {

	// Set plain text header
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	// write the content to socket
	if _, err := w.Write([]byte(data)); err != nil {
		log.Printf("Error writing data to socket: %v", err)
		c.Error500(err, w, r)
	}
}

func (c HTTPController) Error500(err error, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (c HTTPController) BadRequest(err error, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func (c HTTPController) NotFound(err error, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, err.Error(), http.StatusNotFound)
}

func (c HTTPController) Forbidden(err error, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	http.Error(w, err.Error(), http.StatusForbidden)
}

func (C HTTPController) Render(tpl *template.Template, data interface{}, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	tpl.Execute(w, data)
}

func (C HTTPController) Redirect(code int, url string, w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, url, code)
}
