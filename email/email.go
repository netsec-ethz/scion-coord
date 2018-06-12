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

const mailTemplatesPath = "email/templates"

type Email struct {
	From    string
	To      []string
	Subject string
	Body    string
	Tag     string
}

type MailData struct {
	FirstName        string
	LastName         string
	HostAddress      string
	VerificationUUID string
}

func MailTemplatePath(template string) string {
	return filepath.Join(mailTemplatesPath, template)
}

// construct builds an email to the user specified by
// their userEmail by filling in the specified template with the given subject
// and information in the data object
func construct(emailTemplate, subject string, data interface{}, tag, userEmail string) (*Email, error) {
	tmpl, err := template.ParseFiles(MailTemplatePath(emailTemplate))
	if err != nil {
		log.Printf("Parsing template %v failed: %v", emailTemplate, err)
		return nil, err
	}

	buf := new(bytes.Buffer)
	tmpl.Execute(buf, data)
	body := buf.String()

	mail := new(Email)
	mail.From = config.EmailFrom
	mail.To = []string{userEmail}
	mail.Subject = subject
	mail.Body = body
	mail.Tag = tag
	return mail, nil
}

// Send connects to the PostMark email API and sends the email
func Send(mail *Email) error {
	client := postmark.NewClient(config.EmailPMServerToken, config.EmailPMAccountToken)

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

// ConstructAndSendEmail builds and then sends an email. It relies on Construct and Send
func ConstructAndSendEmail(emailTemplate string, subject string, data interface{},
	tag string, userEmail string, alsoToAdmins bool) error {

	mail, err := construct(emailTemplate, subject, data, tag, userEmail)
	if err != nil {
		log.Printf("ConstructAndSend failed in Construct: %v", err)
		return err
	}
	if err = Send(mail); err != nil {
		log.Printf("Sending email to %v failed: %v", userEmail, err)
		return err
	}

	if alsoToAdmins {
		mail.To = config.EmailAdmins
		mail.Subject = "[SCIONLab ADMIN]" + mail.Subject
		if err = Send(mail); err != nil {
			log.Printf("Sending email to %v failed: %v", config.EmailAdmins, err)
			return err
		}
	}

	return nil
}

// ConstructAndSendEmailToAdmins builds and sends an email to the ScionLab admins
func ConstructAndSendEmailToAdmins(emailTemplate string, subject string, data interface{}, tag string) error {
	mail, err := construct(emailTemplate, subject, data, tag, "")
	if err != nil {
		log.Printf("ConstructAndSend failed in Construct: %v", err)
		return err
	}
	mail.To = config.EmailAdmins
	if err = Send(mail); err != nil {
		log.Printf("Sending email to %v failed: %v", config.EmailAdmins, err)
		return err
	}
	return nil
}
