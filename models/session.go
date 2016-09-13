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
