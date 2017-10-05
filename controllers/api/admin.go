// Copyright 2017 ETH Zurich
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
	"log"
	"net/http"

	"github.com/netsec-ethz/scion-coord/controllers"
)

type AdminController struct {
	controllers.HTTPController
}

type adminPageData struct {
	User user
	//EmailTemplate string // option to allow the email template to be changed
}

type invitationInfo struct {
	FirstName    string
	LastName     string
	Email        string
	Organisation string
}

type emailTemplateInfo struct {
	FirstName        string
	LastName         string
	InvitorFirstName string
	InvitorLastName  string
	HostAddress      string
	UUID             string
}

type invitationsData []invitationInfo

func (c AdminController) AdminInformation(w http.ResponseWriter, r *http.Request) {

	user, err := populateUserData(w, r)
	if err != nil {
		log.Println(err)
		c.Forbidden(err, w, r)
		return
	}

	adminData := adminPageData{
		User: user,
	}

	c.JSON(&adminData, w, r)
	return
}
