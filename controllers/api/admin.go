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

package api

import (
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"html/template"
	"net/http"
)

type AdminController struct {
	controllers.HTTPController
}

func (c *AdminController) Index(w http.ResponseWriter, r *http.Request) {

	// get the current user session if present.
	// if not then, abort
	_, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		http.Redirect(w, r, "/login", 302)
		return
	}

	t, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	c.Render(t, nil, w, r)
}
