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

func (reservation *Reservation) Insert() error {

	// TODO: Check, if the as_source has enough credits to make a reservation
	_, err := o.Insert(reservation)
	return err
}

// TODO: Add function to delete old, invalid reservations