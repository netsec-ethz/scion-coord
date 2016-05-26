package api

import (
	"errors"
	//"fmt"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/models"
	//"html"
	//"log"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type As struct {
	Id          uint64
	Certificate string
}

type ASController struct {
	controllers.HTTPController
}

func (c *ASController) Exists(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	asId := vars["as_id"]

	if asId == "" {
		c.BadRequest(errors.New("missing as_id parameter"), w, r)
		return
	}

	id, err := strconv.ParseUint(asId, 10, 64)
	if err != nil {
		c.Error500(err, w, r)
		return
	}

	if id < 0 {
		c.BadRequest(errors.New("Negative values not allowed"), w, r)
		return
	}

	if _, err := models.FindAsById(id); err != nil {
		c.NotFound(asId+" not found", w, r)
		return
	}

	fmt.Fprintln(w, "{}")
}

func (c *ASController) Upsert(w http.ResponseWriter, r *http.Request) {

	var as As
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&as); err != nil {
		c.BadRequest(err, w, r)
		return
	}

	// get the account from the key and secret
	vars := mux.Vars(r)
	key := vars["key"]
	if key == "" {
		key = r.URL.Query().Get("key")
	}

	// should never happen this, but one nevers knows...
	if key == "" {
		c.BadRequest(errors.New("Missing key parameter"), w, r)
		return
	}

	// find the account belonging to the request
	account, err := models.FindAccountByKey(key)
	if err != nil {
		c.Error500(err, w, r)
		return
	}

	// create a new DB model
	finalAs := models.As{
		Id:          as.Id,
		Certificate: as.Certificate,
		Account:     account,
	}

	// upsert it
	if err := finalAs.Upsert(); err != nil {
		c.Error500(err, w, r)
		return
	}

	// we can decide whether to return the data back or a simple 200
	fmt.Fprintln(w, "{}")
}
