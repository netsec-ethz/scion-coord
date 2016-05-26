package models

import (
	"testing"
)

func TestCreateUser(t *testing.T) {
	email := "sebastian.cogno@swisscom.com"
	password := "this is my password nobidy should copy it"

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

	if user.Account.Key == "" {
		t.Error("Empty account key")
	}

	if user.Account.Secret == "" {
		t.Error("Empty account secret")
	}

	// check authentication
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
