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
	"encoding/gob"
)

type Session struct {
	UserId       uint64
	Email        string
	First        string
	Last         string
	Organisation string
	XSRFToken    string
	HasLoggedIn  bool
	Error        string // errors to display while rendering the template
}

type M map[string]interface{}

func init() {

	gob.Register(&Session{})
	gob.Register(&M{})
}
