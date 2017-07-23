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

// Package email provides functionality for constructing and sending emails using a local SMTP server
package email

import (
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"time"
)

type Email struct {
	From    string
	To      []string
	Subject string
	Body    string
}

type SMTPServer struct {
	Host     string
	Port     int
	User     string
	Password string
}

var timeNow = time.Now

// Concatenates the hostname and port to get the servername as needed in the Send function
func (s *SMTPServer) serverName() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// Composes the message to be sent using the fields specified in Email
func (mail *Email) buildMessage() (string, error) {
	if len(mail.To) == 0 {
		return "", errors.New("No recipients specified")
	}
	message := fmt.Sprintf("From: %s\r\n", mail.From)
	message += fmt.Sprintf("To: %s\r\n", strings.Join(mail.To, ","))
	message += fmt.Sprintf("Subject: %s\r\n", mail.Subject)
	message += "Content-Type: text/plain; charset=utf-8\r\n"
	message += "Date :" + fmt.Sprintf(timeNow().Format("02 Jan 2006 15:04:05 -0700")) + "\r\n"
	message += "\r\n" + mail.Body

	return message, nil
}

// Send connects to the specified server and sends the email
func Send(mail *Email, server *SMTPServer) (err error) {

	// Connect to SMTP server
	c, err := smtp.Dial(server.serverName())
	if err != nil {
		log.Printf("Error connecting to SMTP server: %v", err)
		return
	}

	// Set the sender and recipients
	if err = c.Mail(mail.From); err != nil {
		log.Printf("Error setting sender: %v", err)
		return
	}

	for _, t := range mail.To {
		if err = c.Rcpt(t); err != nil {
			log.Printf("Error setting recipients: %v", err)
			return
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		log.Printf("Error sending the DATA command: %v", err)
		return
	}

	// build the message
	message, err := mail.buildMessage()
	if err != nil {
		log.Printf("Error building message: %v", err)
		return
	}

	// write the message
	if _, err = wc.Write([]byte(message)); err != nil {
		log.Printf("Error writing email body: %v", err)
		return
	}

	// close the writer
	if err = wc.Close(); err != nil {
		log.Printf("Error closing WriteCloser: %v", err)
		return
	}

	// quit connection with server
	if err = c.Quit(); err != nil {
		log.Printf("Error closing connection with SMTP server: %v", err)
		return
	}

	return

}
