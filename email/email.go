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
	"bytes"
	"html/template"
	"log"
	"path/filepath"
	"strings"

	"github.com/keighl/postmark"
	"github.com/netsec-ethz/scion-coord/config"
)

var EMAIL_TEMPLATES_PATH = "email/templates"

type Email struct {
	From    string
	To      []string
	Subject string
	Body    string
	Tag     string
}

type EmailData struct {
	FirstName        string
	LastName         string
	Protocol         string
	HostAddress      string
	VerificationUUID string
}

func EmailTemplatePath(template string) string {
	return filepath.Join(EMAIL_TEMPLATES_PATH, template)
}

// Send connects to the PostMark email API and sends the email
func Send(mail *Email) error {

	client := postmark.NewClient(config.EMAIL_PM_SERVER_TOKEN, config.EMAIL_PM_ACCOUNT_TOKEN)

	email := postmark.Email{
		From:       mail.From,
		To:         strings.Join(mail.To, ","),
		Subject:    mail.Subject,
		TextBody:   mail.Body,
		Tag:        mail.Tag,
		TrackOpens: true,
	}

	_, err := client.SendEmail(email)

	if err != nil {
		log.Printf("Error sending email via PostMark to: %v, %v", email.To, err)
		return err
	}

	return nil

}

// ConstructAndSend builds and then sends an email to the user specified by
// their userEmail by filling in the specified template with the given subject
// and information in the data object
func ConstructAndSend(emailTemplate string, subject string, data interface{},
	tag string, userEmails ...string) (err error) {

	tmpl, err := template.ParseFiles(EmailTemplatePath(emailTemplate))
	if err != nil {
		log.Printf("Parsing template %v failed: %v", emailTemplate, err)
		return err
	}

	buf := new(bytes.Buffer)
	tmpl.Execute(buf, data)
	body := buf.String()

	mail := new(Email)
	mail.From = config.EMAIL_FROM
	mail.To = userEmails
	mail.Subject = subject
	mail.Body = body
	mail.Tag = tag

	if err = Send(mail); err != nil {
		log.Printf("Sending email to %v failed: %v", userEmails, err)
		return err
	}

	return nil
}
