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

package geolocation

import (
	"log"
	"net"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/oschwald/geoip2-golang"
	"github.com/scionproto/scion/go/lib/addr"
)

// IP address to ISO country & continent
func IPGeolocation(ipAddress string) (country string, continent string, err error) {
	db, err := geoip2.Open("utility/geolocation/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// If you are using strings that may be invalid, check that ip is not nil
	ip := net.ParseIP(ipAddress)
	record, err := db.City(ip)
	return record.Country.IsoCode, record.Continent.Code, err
}

// Get the ISD from ISO country & continent
func Location2ISD(country string, continent string) (addr.ISD, error) {
	// look for isd in country
	result, err := models.FindISDbyCountry(country)
	if err != nil {
		if err == orm.ErrNoRows {
			// look for isd in continent
			result, err = models.FindISDbyContinent(continent)
			if err != nil {
				if err == orm.ErrNoRows {
					// no location found return default ISD 1
					return 1, nil
				} else {
					return 0, err
				}
			}
		} else {
			return 0, err
		}
	}
	return addr.ISD(result.ISD), err
}
