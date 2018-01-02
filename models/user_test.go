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

package models

import (
	"testing"

	"github.com/netsec-ethz/scion-coord/config"
)

func TestCreateUser(t *testing.T) {
	email := "sebastian.cogno@swisscom.com"
	password := "this is my password nobody should copy it"

	_, err := RegisterUser("scion", "Scion Test-Bed", email, password, "Jon", "Doe")
	if err != nil {
		t.Fatal(err)
	}

	// try to retrieve the data
	user, err := FindUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}

	if user == nil {
		t.Error("Could not find user")
	}

	if user.Email == "" {
		t.Error("Empty email")
	}

	if user.FirstName == "" {
		t.Error("Empty FirstName")
	}

	if user.LastName == "" {
		t.Error("Empty LastName")
	}

	if user.Password == "" {
		t.Error("Empty Password")
	}

	if user.Account.Name == "" {
		t.Error("Empty account name")
	}

	if user.Account.Organisation == "" {
		t.Error("Empty account org")
	}

	if user.Account.AccountId == "" {
		t.Error("Empty AccountId ")
	}

	if user.Account.Secret == "" {
		t.Error("Empty account secret")
	}

	// switch on user activation for testing
	config.USER_ACTIVATION = true

	// check authentication
	if err := user.Authenticate(password); err.Error() != "Email is not verified" {
		t.Error(err)
	}

	// verify user and authenticate again
	user.UpdateVerified(true)
	if err := user.Authenticate(password); err.Error() != "User is not activated" {
		t.Error(err)
	}

	// Activate user and authenticate again
	user.UpdateActivated(true)
	if err := user.Authenticate(password); err != nil {
		t.Error("Could not authenticate")
	}

	// fail authentication purposely
	if err := user.Authenticate("1234"); err == nil {
		t.Error("Open authentication !")
	}

	// clean up
	if err := user.Account.Delete(); err != nil {
		t.Fatal(err)
	}

	if err := user.Delete(); err != nil {
		t.Fatal(err)
	}

}
