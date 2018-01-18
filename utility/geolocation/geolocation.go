package geolocation

import (
	"log"
	"net"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/oschwald/geoip2-golang"
)

// IP address to ISO country & continent
func IP_geolocation(IP_address string) (country string, continent string, err error) {
	db, err := geoip2.Open("utility/geolocation/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// If you are using strings that may be invalid, check that ip is not nil
	ip := net.ParseIP(IP_address)
	record, err := db.City(ip)
	return record.Country.IsoCode, record.Continent.Code, err
}

// Get the ISD from ISO country & continent
func Location2Isd(country string, continent string) (int, error) {
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
					return -1, err
				}
			}
		} else {
			return -1, err
		}
	}
	return result.ISD, err
}

func GetCity(record *geoip2.City) (city string, err error) {

	return record.Subdivisions[0].Names["en"], err
}

func GetCountry(record *geoip2.City) (country string, err error) {

	return record.Continent.Names["en"], err
}

func GetCountry_ISO(record *geoip2.City) (country_iso string, err error) {

	return record.Country.IsoCode, err
}

func GetContinent(record *geoip2.City) (continent string, err error) {

	return record.Continent.Names["en"], err
}

func getContinent_Code(record *geoip2.City) (continent_code string, err error) {

	return record.Continent.Code, err
}

func getLatitude(record *geoip2.City) (latitude float64, err error) {

	return record.Location.Latitude, err
}

func getLongitude(record *geoip2.City) (longitude float64, err error) {

	return record.Location.Longitude, err
}
