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
	"testing"
)

func TestBuildMessage(t *testing.T) {

	mail := new(Email)
	mail.From = "sender@test.com"
	mail.To = []string{"receiver@example.com"}
	mail.Subject = "Testing emails"
	mail.Body = "This is a test"

	server := new(SMTPServer)
	server.Host = "mail.server.com"
	server.Port = 25

	// Testing buildMessage with one receiver
	if message := mail.buildMessage(); message !=
		"From: sender@test.com\r\n"+
			"To: receiver@example.com\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html\r\n"+
			"Subject: Testing emails\r\n"+
			"\r\n"+
			"This is a test" {

		t.Error("Error building message for one receiver")
	}

	// Testing buildMessage with multiple receivers
	mail.To = append(mail.To, "receiver2@domain2.com", "receiver3@domain3.com", "receiver4@domain4.com", "receiver5@domain5.com")
	if message := mail.buildMessage(); message !=
		"From: sender@test.com\r\n"+
			"To: receiver@example.com,receiver2@domain2.com,receiver3@domain3.com,receiver4@domain4.com,receiver5@domain5.com\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/html\r\n"+
			"Subject: Testing emails\r\n"+
			"\r\n"+
			"This is a test" {

		t.Error("Error building message for multiple receivers")
	}

	// Testing serverName function
	if name := server.serverName(); name != "mail.server.com:25" {
		t.Error("Error building servername")
	}

}
