package api

import (
	"github.com/netsec-ethz/scion-coord/controllers"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/models"
	"fmt"
	"encoding/json"
	"log"
	"errors"
	"github.com/netsec-ethz/scion/go/lib/addr"
	"time"
)

type CurrencyController struct {
	controllers.HTTPController
}

type ReservationInfo struct {
	Id        uint64
	Time      time.Time
	Amount    uint32
}

type ReservationRequest struct {
	SourceIsd int
	SourceAs  int
	TargetIsd int
	TargetAs  int
	Time      time.Time
	Amount    uint32
}


func (c *CurrencyController) GetValidReservation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	isdasString := vars["isd-as"]

	if _, err := models.FindAsByIsdAs(isdasString); err != nil {
		c.NotFound(errors.New(isdasString+" not found"), w, r)
		return
	}
	isdas,_ := addr.IAFromString(isdasString)

	reservations,_ := models.FindReservationsBySource(*isdas)
	var reservationInfo [len(reservations)]ReservationInfo
	for i,p := range reservations {
		reservationInfo[i] = ReservationInfo{
			Id: p.Id,
			Time: p.Time,
			Amount: p.Amount,
		}
	}
	b, err := json.Marshal(reservationInfo)
	if err != nil {
		log.Printf("Error marshaling JSON for reservationInfo: %v %v",
			reservations, err)
		c.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

func (c *CurrencyController) PostCreateReservation(w http.ResponseWriter, r *http.Request) {
	var cr ReservationRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&cr); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		c.BadRequest(err, w, r)
		return
	}

	reservation := models.Reservation {
		SourceIsd: cr.SourceIsd,
		SourceAs: cr.SourceAs,
		TargetIsd: cr.TargetIsd,
		TargetAs: cr.TargetAs,
		Time: cr.Time,
		Amount : cr.Amount,
	}
	err := reservation.Insert()
	if err != nil {
		log.Printf("Error inserting reservation: %v %v",
			reservation, err)
		c.Error500(err, w, r)
		return
	}
	// TODO: Return the reservation ID
}
