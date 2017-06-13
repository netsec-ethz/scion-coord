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

package email

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
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

// Concatenates the hostname and port to get the servername as needed in the Send function
func (s *SMTPServer) serverName() string {
	return s.Host + ":" + fmt.Sprintf("%d", s.Port)
}

// Composes the message to be sent using the fields specified in Email
func (mail *Email) buildMessage() string {
	message := ""
	message += fmt.Sprintf("From: %s\r\n", mail.From)
	if len(mail.To) > 0 {
		message += fmt.Sprintf("To: %s\r\n", strings.Join(mail.To, ","))
	}
	message += "MIME-Version: 1.0\r\nContent-Type: text/html\r\n"
	message += fmt.Sprintf("Subject: %s\r\n", mail.Subject)
	message += "\r\n" + mail.Body

	return message
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

	//write the message
	if _, err = wc.Write([]byte(mail.buildMessage())); err != nil {
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
