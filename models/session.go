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
}

type M map[string]interface{}

func init() {

	gob.Register(&Session{})
	gob.Register(&M{})
}
