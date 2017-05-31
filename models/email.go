package models

import (
	"fmt"
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

func (s *SMTPServer) serverName() string {
	return s.Host + ":" + fmt.Sprintf("%d", s.Port)
}

func (mail *Email) buildMessage() string {
	message := ""
	message += fmt.Sprintf("From: %s\r\n", mail.From)
	if len(mail.To) > 0 {
		message += fmt.Sprintf("To: %s\r\n", strings.Join(mail.To, ","))
	}

	message += fmt.Sprintf("Subject: %s\r\n", mail.Subject)
	message += "\r\n" + mail.Body

	return message
}

func Send(mail *Email, server *SMTPServer) (err error) {

	/* use this for authentication

	//Setting up TLS connection
	tlsconf := &tls.Config{ServerName: server.host,
		InsecureSkipVerify: true}

	con, err := tls.Dial("tcp", server.host(), tlsconf)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(con, server.host)
	if err != nil {
		return err
	}

	auth := smtp.PlainAuth("", server.User, server.Password, server.host)

	// Authenticate
	if err = c.Auth(auth); err != nil {
		return err
	}

	*/

	// Connect to SMTP server
	c, err := smtp.Dial(server.serverName())
	if err != nil {
		return
	}

	// Set the sender and recipients
	if err = c.Mail(mail.From); err != nil {
		return
	}

	for _, t := range mail.To {
		if err = c.Rcpt(t); err != nil {
			return
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return
	}

	if _, err = wc.Write([]byte(mail.buildMessage())); err != nil {
		return
	}

	if err = wc.Close(); err != nil {
		return
	}

	if err = c.Quit(); err != nil {
		fmt.Printf("%s", err)
		return
	}

	return nil

}
