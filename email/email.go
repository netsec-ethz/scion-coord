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

// Package email provides functionality for constructing and sending emails via PostMark
package email

import (
	"github.com/keighl/postmark"
	"github.com/netsec-ethz/scion-coord/config"
	"log"
	"strings"
)

type Email struct {
	From    string
	To      []string
	Subject string
	Body    string
}

// Send connects to the PostMark email API and sends the email
func Send(mail *Email) error {

	client := postmark.NewClient(config.EMAIL_PM_SERVER_TOKEN, config.EMAIL_PM_ACCOUNT_TOKEN)

	email := postmark.Email{
		From:       mail.From,
		To:         strings.Join(mail.To, ","),
		Subject:    mail.Subject,
		TextBody:   mail.Body,
		Tag:        "email-verification",
		TrackOpens: true,
	}

	_, err := client.SendEmail(email)

	if err != nil {
		log.Printf("Error during send email: %v", err)
		return err
	}

	return nil

}
