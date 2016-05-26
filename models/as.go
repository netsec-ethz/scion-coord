package models

import (
	"crypto/x509"
	"encoding/pem"
	"time"
)

type As struct {
	Id          uint64
	AsId        uint64   `orm:"index"`
	Certificate string   `orm:"type(text)"`
	Account     *account `orm:"rel(fk);index"`
	Created     time.Time
	Updated     time.Time
}

func FindAsById(id uint64) (*As, error) {
	as := new(As)
	err := o.QueryTable(as).Filter("Id", id).RelatedSel().One(as)
	return as, err
}

func (as *As) deleteAs() error {
	_, err := o.Delete(as)
	return err
}

func (as *As) Upsert() error {
	storedAs, err := FindAsById(as.Id)
	if err == nil && storedAs != nil && storedAs.Id > 0 {
		storedAs.Id = as.Id
		storedAs.Certificate = as.Certificate
		as.Updated = time.Now().UTC()
		_, err := o.Update(as)
		return err
	}

	_, err = o.Insert(as)
	return err
}

// This function is a placeholder for verifying that the certificate contains the correct AS id
func (as *As) verifyCertificate() {
	// First, create the set of root certificates. For this example we only
	// have one. It's also possible to omit this in order to use the
	// default root set of the current operating system.
	roots := x509.NewCertPool()
	// ok := roots.AppendCertsFromPEM([]byte(rootPEM))
	// if !ok {
	// 	panic("failed to parse root certificate")
	// }

	block, _ := pem.Decode([]byte(as.Certificate))
	if block == nil {
		panic("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic("failed to parse certificate: " + err.Error())
	}

	opts := x509.VerifyOptions{
		DNSName: "mail.google.com",
		Roots:   roots,
	}

	if _, err := cert.Verify(opts); err != nil {
		panic("failed to verify certificate: " + err.Error())
	}
}
