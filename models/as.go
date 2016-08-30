package models

import (
	"strconv"
	"strings"
	"time"
)

type As struct {
	IsdAs   string   `orm:"index;pk"`
	Isd     uint64   `orm:"index"`
	Core    bool     `orm:"default(false)"`
	Account *Account `orm:"rel(fk);index"`
	Created time.Time
}

func FindCoreAsByIsd(isd uint64) ([]As, error) {
	var ases []As
	_, err := o.QueryTable("as").Filter("Isd", isd).Filter("Core", true).All(&ases)
	return ases, err
}

func FindAsByIsdAs(isd_as string) (*As, error) {
	as := new(As)
	err := o.QueryTable(as).Filter("IsdAs", isd_as).RelatedSel().One(as)
	return as, err
}

func AllASes() ([]As, error) {
	var ASes []As
	_, err := o.QueryTable("as").All(&ASes)
	return ASes, err
}

func (as *As) deleteAs() error {
	_, err := o.Delete(as)
	return err
}

func (as *As) Insert() error {
	_, err := o.Insert(as)
	return err
}

func IsdAsToIsd(isdas string) (uint64, error) {
	return strconv.ParseUint(strings.Split(isdas, "-")[0], 10, 64)
}

func FindNumAsInIsd(isd uint64) (int64, error) {
	return o.QueryTable("as").Filter("Isd", isd).Count()
}
