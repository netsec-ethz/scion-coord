package models

import (
	"github.com/netsec-ethz/scion/go/lib/addr"
	"time"
)

type Reservation struct {
	Id        uint64    `orm:"column(id);auto;pk"`
	SourceIsd int      `orm:"index"`
	SourceAs  int      `orm:"index"`
	TargetIsd int      `orm:"index"`
	TargetAs  int      `orm:"index"`
	Time      time.Time
	Amount    uint32
}

func FindReservationsBySource(source addr.ISD_AS) ([]Reservation, error) {

	var reservations []Reservation
	standardDuration := 16 * time.Second
	_, err := o.QueryTable("reservation",
	).Filter("SourceIsd", source.I).Filter("SourceAs", source.A, // Filter all reservation BY an ISD-AS
	).Filter("time__gte", time.Now().Add(-standardDuration), // Filter all reservation which are valid (-16 seconds from NOW())
	).All(&reservations)
	return reservations, err
}

func FindTodaysReservationsBySource(source addr.ISD_AS) ([]Reservation, error) {

	var now,today,tommorow
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tommorow := time.AddDate(0,0,1)
	var reservations []Reservation
	standardDuration := 16 * time.Second
	_, err := o.QueryTable("reservation",
	).Filter("SourceIsd", source.I).Filter("SourceAs", source.A, // Filter all reservation BY an ISD-AS
      	).Filter("time__gte", today).Filter("time__lte", tommorow, // Filter all reservation which are valid (-16 seconds from NOW())
	).All(&reservations)
	return reservations, err
}

func (reservation *Reservation) Insert() error {
	var reservations []Reservation
	// TODO: Make the interval variable
	reservations, err := FindTodaysReservationsBySource(addr.ISD_AS(reservation.SourceIsd,reservation.SourceAs))
	if err != nill
		return err
	var creditsToday = reservation.Amount / 10
	for i, v := range reservations {
		creditsToday += v.Amount / 10
	}
	
	
	as, err := models.FindAsByIsdAs(reservation.SourceIsd + "-"+reservation.SourceAs)
	if as.Credits <= creditsToday {
		err := errors.New("no more credits for today")
		return err
	}
	_, err := o.Insert(reservation)
	
	return err
}

// TODO: Add function to delete old, invalid reservations
