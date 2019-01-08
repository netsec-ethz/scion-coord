// Copyright 2016 ETH Zurich
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

package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/sec51/goconf"
)

// Settings are specified in conf/development.conf
var (
	// address the service listens on
	HTTPBindAddress = goconf.AppConf.String("http.bind_address")
	// port the service listens on
	HTTPBindPort, _ = goconf.AppConf.Int("http.bind_port")
	// address from which the service is reachable from outside
	HTTPHostAddress        = goconf.AppConf.String("http.host_address")
	HTTPEnableHTTPS, _     = goconf.AppConf.Bool("http.enable_https")
	EmailPMServerToken     = goconf.AppConf.String("email.pm_server_token")
	EmailPMAccountToken    = goconf.AppConf.String("email.pm_account_token")
	EmailFrom              = goconf.AppConf.String("email.from")
	EmailAdmins            = goconf.AppConf.Strings("email.admin_emails")
	CaptchaSecretKey       = goconf.AppConf.String("captcha.secret_key")
	CaptchaSiteKey         = goconf.AppConf.String("captcha.site_key")
	SessionPath            = goconf.AppConf.String("session.path")
	SessionEncryptionKey   = goconf.AppConf.String("session.encryption_key")
	SessionVerificationKey = goconf.AppConf.String("session.verification_key")
	LogFile                = goconf.AppConf.String("log.file")
	LogDebugMode, _        = goconf.AppConf.Bool("log.debug_mode")
	PackageDirectory       = goconf.AppConf.DefaultString("directory.package_directory",
		filepath.Join(os.Getenv("HOME"), "scionLabConfigs"))
	ISDLocationMapping           = goconf.AppConf.String("directory.isd_location_map")
	DBName                       = goconf.AppConf.String("db.name")
	DBHost                       = goconf.AppConf.String("db.host")
	DBPort, _                    = goconf.AppConf.Int("db.port")
	DBUser                       = goconf.AppConf.String("db.user")
	DBPassword                   = goconf.AppConf.String("db.pass")
	DBMaxConnections, _          = goconf.AppConf.Int("db.max_connections")
	DBMaxIdle, _                 = goconf.AppConf.Int("db.max_idle")
	BaseASID                     addr.AS
	MaxBRID, _                   = goconf.AppConf.Int("max_br_id")
	ReservedBRsInfrastructure, _ = goconf.AppConf.Int("reserved_brs_infrastructure")
	ASesPerUser, _               = goconf.AppConf.Int("ases_per_user")
	ASesPerAdmin, _              = goconf.AppConf.Int("ases_per_admin")
	SigningASes                  = make(map[addr.ISD]addr.AS) // map[ISD]=signing_as
	MTU, _                       = goconf.AppConf.Int("mtu")
	BRStartPort                  uint16
	BRInternalStartPort          uint16

	// Virtual Credit system
	VirtualCreditEnable, _       = goconf.AppConf.Bool("virtualCredit.enable")
	VirtualCreditStartCredits, _ = goconf.AppConf.Int64("virtualCredit.startCredits")

	// Local IP address in VM; this is a default set by
	// vagrant and may have to be adjusted if vagrant configuration is changed
	VMLocalIP          = "10.0.2.15"
	LocalhostIP        = "127.0.0.1"
	HeartbeatPeriod, _ = goconf.AppConf.Int("heartbeat.period")
	HeartbeatLimit, _  = goconf.AppConf.Int("heartbeat.limit")

	GrafanaURL   = goconf.AppConf.String("grafana.url")
	TutorialsURL = goconf.AppConf.String("tutorials.url")

	// Image building service information
	IMGBuilderAddressPublic   = goconf.AppConf.String("img_builder.address.public")
	IMGBuilderAddressInternal = goconf.AppConf.String("img_builder.address.internal")
	IMGBuilderSecretToken     = goconf.AppConf.String("img_builder.secret_token")
	IMGBuilderBuildDelay, _   = goconf.AppConf.Int64("img_builder.build_delay")
)

func init() {
	sp := goconf.AppConf.DefaultInt("br_bind_start_port", 50000)
	BRStartPort = uint16(sp) // Ports are only 16 bits
	sp = goconf.AppConf.DefaultInt("br_internal_start_port", 31046)
	BRInternalStartPort = uint16(sp) // Ports are only 16 bits
	signingMap, err := goconf.AppConf.GetSection("signing_ases")
	if err != nil {
		fmt.Println("Error reading configuration for signing_ases:", err)
		os.Exit(1)
	}
	for k, v := range signingMap {
		ki, err := strconv.Atoi(k)
		if err != nil {
			fmt.Println("Error parsing section signing_ases:", err)
			os.Exit(1)
		}
		if ki < 1 || ki > addr.MaxISD {
			fmt.Println("Invalid value for ISD: ", k)
			os.Exit(1)
		}

		var asID addr.AS
		vi, err := strconv.Atoi(v)
		if err != nil {
			asID, err = addr.ASFromString(v)
			if err != nil {
				fmt.Println("Error parsing section signing_ases:", err)
				os.Exit(1)
			}
		} else {
			asID = addr.AS(vi)
		}
		SigningASes[addr.ISD(ki)] = asID
	}
	auxInt, err := goconf.AppConf.Int64("base_as_id")
	if err != nil {
		auxString := goconf.AppConf.String("base_as_id")
		BaseASID, err = addr.ASFromString(auxString)
		if err != nil {
			BaseASID = addr.AS(utility.ScionlabUserASOffsetAddr)
			log.Printf("Config: not a valid AS id: '%v'. Using %v as base instead.", auxString, BaseASID.String())
		}
	} else {
		BaseASID = addr.AS(auxInt)
	}
	fmt.Println("Base AS ID: ", BaseASID.String())

	// we don't validate the email addresses, we just trim them in case they had leading/trailing spaces
	for i, admin := range EmailAdmins {
		EmailAdmins[i] = strings.Trim(admin, " ")
	}

	if flag.Lookup("test.v") != nil {
		// this is running in testing, chdir to the right place
		_, spath, _, _ := runtime.Caller(0)
		spath = filepath.Dir(spath)
		spath = filepath.Dir(spath)
		err = os.Chdir(spath)
		if err != nil {
			fmt.Printf("Error in test chdir to %v: %v", spath, err)
			os.Exit(1)
		}
	}
}

func MaxASes(isAdmin bool) int {
	if isAdmin {
		return ASesPerAdmin
	}
	return ASesPerUser
}
