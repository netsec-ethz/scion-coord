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

package email

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	mail   = Email{From: "sender@test.com", To: []string{"receiver@example.com"}, Subject: "Testing emails", Body: "This is a test"}
	server = SMTPServer{Host: "mail.server.com", Port: 25}
)

func init() {
	timeNow = func() time.Time {
		t, _ := time.Parse("2006-01-02 15:04:05", "2017-01-20 01:02:03")
		return t
	}
}

func TestBuildMessageSingle(t *testing.T) {

	// Testing buildMessage with one receiver
	message, _ := mail.buildMessage()
	assert.Equal(t,
		"From: sender@test.com\r\n"+
			"To: receiver@example.com\r\n"+
			"Subject: Testing emails\r\n"+
			"Content-Type: text/plain; charset=utf-8\r\n"+
			"Date :"+fmt.Sprintf(timeNow().Format("02 Jan 2006 15:04:05 -0700"))+"\r\n"+
			"\r\n"+
			"This is a test", message)
}

func TestBuildMessageMulti(t *testing.T) {

	// Testing buildMessage with multiple receivers
	mail.To = append(mail.To, "receiver2@domain2.com", "receiver3@domain3.com", "receiver4@domain4.com", "receiver5@domain5.com")
	message, _ := mail.buildMessage()
	assert.Equal(t,
		"From: sender@test.com\r\n"+
			"To: receiver@example.com,receiver2@domain2.com,receiver3@domain3.com,receiver4@domain4.com,receiver5@domain5.com\r\n"+
			"Subject: Testing emails\r\n"+
			"Content-Type: text/plain; charset=utf-8\r\n"+
			"Date :"+fmt.Sprintf(timeNow().Format("02 Jan 2006 15:04:05 -0700"))+"\r\n"+
			"\r\n"+
			"This is a test", message)
}

func TestServerName(t *testing.T) {

	// Testing serverName function
	name := server.serverName()
	assert.Equal(t, "mail.server.com:25", name)
}
