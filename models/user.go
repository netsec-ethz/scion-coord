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
	//"crypto"
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/astaxie/beego/orm"
	uuid "github.com/pborman/uuid"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/scrypt"
	"time"
)

const (
	API_CONTEXT   = string("scion-coordinator")
	SALT_LENGTH   = 80
	SECRET_LENGTH = 32
)

type Account struct {
	Id           uint64
	Name         string
	Organisation string
	AccountId    string
	Secret       string
	Users        []*user `orm:"reverse(many);index"`
	ASes         []*As   `orm:"reverse(many);index"`
	Created      time.Time
	Updated      time.Time
}

type user struct {
	Id        uint64
	Email     string `orm:"index"`
	Password  string
	Salt      string
	FirstName string
	LastName  string
	Verified  bool     // whether the user verified the email
	Account   *Account `orm:"rel(fk);index"`
	Created   time.Time
	Updated   time.Time
	// TODO: add the 2 factor authentication
}

func generateSalt() ([]byte, error) {
	salt := make([]byte, SALT_LENGTH)
	var saltErr error
	var total int

	for i := 0; i < 10; i++ {
		total, saltErr = rand.Read(salt)
		if saltErr == nil && total == SALT_LENGTH {
			return salt, nil
		}
	}

	return salt, saltErr
}

func derivePassword(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
}

// This function creates both a new user and a new account and associate them
func RegisterUser(accountName, organisation, email, password, first, last string) (*user, error) {

	// find whether the user email is already taken
	storedUser, err := FindUserByEmail(email)

	if err == nil && storedUser != nil && storedUser.Id > 0 {
		return nil, errors.New("User already registered")
	}

	// generate a random salt when the user registers the first time
	salt, saltError := generateSalt()

	// in case of errors generating the salt DO NOT PROCEED !
	if saltError != nil {
		return nil, saltError
	}

	// Derive a password using scrypt with
	derivedPassword, scryptErr := derivePassword(password, salt)

	// means this is a new user
	// 1 - create account
	// 2 - create user
	// 3 - link user to account
	if err == orm.ErrNoRows {
		// 1 check if an account already exists
		a, err := FindAccount(accountName)

		// if there is no account with the name then create it
		if err == orm.ErrNoRows {

			// Generate the accountId and the secret
			apiSecretReader := hkdf.New(sha256.New, derivedPassword, salt, []byte(API_CONTEXT))
			apiSecretBytes, apiSecretError := bufio.NewReader(apiSecretReader).Peek(SECRET_LENGTH)

			if apiSecretError != nil {
				return nil, apiSecretError
			}

			a = new(Account)
			a.Organisation = organisation
			a.Name = accountName
			a.Created = time.Now().UTC()
			a.Updated = time.Now().UTC()
			a.AccountId = uuid.New()
			a.Secret = hex.EncodeToString(apiSecretBytes)
			if err := a.Upsert(); err != nil {
				return nil, err
			}
		}

		// if there is an error deriving the password then DO NOT PROCEED
		if scryptErr != nil {
			return nil, scryptErr
		}

		// create user
		u := new(user)
		u.Email = email
		u.FirstName = first
		u.LastName = last
		u.Password = hex.EncodeToString(derivedPassword)
		u.Salt = hex.EncodeToString(salt)
		//u.TwoFA = false // set it to false
		u.Created = time.Now().UTC()
		u.Updated = time.Now().UTC()
		//u.LastLoginAttempt = time.Now().UTC()
		// assign user
		u.Account = a
		u.Created = time.Now().UTC()

		_, err = o.Insert(u)
		return u, err

	}

	return nil, errors.New("Unknown error while registering a new user")
}

func (a *Account) Upsert() error {
	storedAccount, err := FindAccount(a.Name)
	if err == nil && storedAccount != nil && storedAccount.Id > 0 {
		a.Id = storedAccount.Id
		a.Updated = time.Now().UTC()
		_, err := o.Update(a)
		return err
	}

	_, err = o.Insert(a)
	return err
}

func FindAccount(name string) (*Account, error) {
	a := new(Account)
	err := o.QueryTable(a).Filter("Name", name).RelatedSel().One(a)
	return a, err
}

func FindUserByEmail(email string) (*user, error) {
	u := new(user)
	err := o.QueryTable(u).Filter("Email", email).RelatedSel().One(u)
	return u, err
}

func FindUserByRecoveryToken(token string) (*user, error) {
	u := new(user)
	err := o.QueryTable(u).Filter("RecoveryToken", token).RelatedSel().One(u)
	return u, err
}

func FindUserEmailToken(token string) (*user, error) {
	u := new(user)
	err := o.QueryTable(u).Filter("EmailToken", token).RelatedSel().One(u)
	return u, err
}

func FindUserById(id string) (*user, error) {
	u := new(user)
	err := o.QueryTable(u).Filter("Id", id).RelatedSel().One(u)
	return u, err
}

func FindUserByAccountIdSecret(acc_id, secret string) (*Account, error) {
	u := new(Account)
	err := o.QueryTable(u).Filter("AccountId", acc_id).Filter("Secret", secret).One(u)
	return u, err
}

func FindAccountByAccountId(acc_id string) (*Account, error) {
	u := new(Account)
	err := o.QueryTable(u).Filter("AccountId", acc_id).One(u)
	return u, err
}

func (u *user) Delete() error {
	_, err := o.Delete(u)
	return err
}

func (a *Account) Delete() error {
	_, err := o.Delete(a)
	return err
}

func (u *user) Authenticate(password string) error {

	// if u.Locked {
	// 	return errors.New("User locked.")
	// }

	// the user did less than N login attempts
	//if u.FailedAttempts <= MAX_LOGIN_ATTEMPTS {
	return u.checkPassword(password)
	//}

	// this means the user tried to log in more than 15 minutes ago
	// if validLockDownWindow(u.LastLoginAttempt) {
	// 	return u.checkPassword(password)
	// }

	// return errors.New("Too many login attempts. Account locked.")
}

func (u *user) checkPassword(password string) error {
	// update time of attempts
	// if err := u.UpdateLastLoginAttempt(); err != nil {
	// 	return err
	// }

	valid := validUserPassword(u.Password, u.Salt, password)
	if valid {
		// if err := u.ResetFailedAttempts(); err != nil {
		// 	return err
		// }
		return nil // means the user is successfully authenticated !
	}

	// // update amount of attempts
	// if err := u.UpdateFailedAttempts(); err != nil {
	// 	return err
	// }

	return errors.New("Failed to login")
}

func validUserPassword(storedPassHex, storedSaltHex, password string) bool {

	// decode the salt from HEX to bytes
	storedSalt, saltDecodeErr := hex.DecodeString(storedSaltHex)
	if saltDecodeErr != nil {
		//log.Println(saltDecodeErr)
		return false
	}

	storedPass, passDecodeErr := hex.DecodeString(storedPassHex)
	if passDecodeErr != nil {
		//log.Println(passDecodeErr)
		return false
	}

	// calculate the HASH based on the SALT and the user input password
	derivedPass, derivedPassErr := derivePassword(password, storedSalt)
	if derivedPassErr != nil {
		//log.Println(derivedPassErr)
		return false
	}

	return bytes.Equal(derivedPass, storedPass)
}
