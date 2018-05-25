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
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sec51/goconf"
)

// Settings are specified in conf/development.conf
var (
	// address the service listens on
	HTTP_BIND_ADDRESS = goconf.AppConf.String("http.bind_address")
	// port the service listens on
	HTTP_BIND_PORT, _ = goconf.AppConf.Int("http.bind_port")
	// address from which the service is reachable from outside
	HTTP_HOST_ADDRESS        = goconf.AppConf.String("http.host_address")
	HTTP_ENABLE_HTTPS, _     = goconf.AppConf.Bool("http.enable_https")
	EMAIL_PM_SERVER_TOKEN    = goconf.AppConf.String("email.pm_server_token")
	EMAIL_PM_ACCOUNT_TOKEN   = goconf.AppConf.String("email.pm_account_token")
	EMAIL_FROM               = goconf.AppConf.String("email.from")
	CAPTCHA_SECRET_KEY       = goconf.AppConf.String("captcha.secret_key")
	CAPTCHA_SITE_KEY         = goconf.AppConf.String("captcha.site_key")
	SESSION_PATH             = goconf.AppConf.String("session.path")
	SESSION_ENCRYPTION_KEY   = goconf.AppConf.String("session.encryption_key")
	SESSION_VERIFICATION_KEY = goconf.AppConf.String("session.verification_key")
	LOG_FILE                 = goconf.AppConf.String("log.file")
	LOG_DEBUG_MODE, _        = goconf.AppConf.Bool("log.debug_mode")
	PACKAGE_DIRECTORY        = goconf.AppConf.DefaultString("directory.package_directory",
		filepath.Join(os.Getenv("HOME"), "scionLabConfigs"))
	ISD_LOCATION_MAPPING           = goconf.AppConf.String("directory.isd_location_map")
	DB_NAME                        = goconf.AppConf.String("db.name")
	DB_HOST                        = goconf.AppConf.String("db.host")
	DB_PORT, _                     = goconf.AppConf.Int("db.port")
	DB_USER                        = goconf.AppConf.String("db.user")
	DB_PASS                        = goconf.AppConf.String("db.pass")
	DB_MAX_CONNECTIONS, _          = goconf.AppConf.Int("db.max_connections")
	DB_MAX_IDLE, _                 = goconf.AppConf.Int("db.max_idle")
	BASE_AS_ID, _                  = goconf.AppConf.Int("base_as_id")
	MAX_BR_ID, _                   = goconf.AppConf.Int("max_br_id")
	RESERVED_BRS_INFRASTRUCTURE, _ = goconf.AppConf.Int("reserved_brs_infrastructure")
	ASES_PER_USER, _               = goconf.AppConf.Int("ases_per_user")
	ASES_PER_ADMIN, _              = goconf.AppConf.Int("ases_per_admin")
	SIGNING_ASES                   = make(map[int]int) // map[ISD]=signing_as
	MTU, _                         = goconf.AppConf.Int("mtu")
	BR_START_PORT                  uint16
	BR_INTERNAL_START_PORT         uint16

	// Virtual Credit system
	VIRTUAL_CREDIT_ENABLE, _        = goconf.AppConf.Bool("virtualCredit.enable")
	VIRTUAL_CREDIT_START_CREDITS, _ = goconf.AppConf.Int64("virtualCredit.startCredits")

	// Local IP address in VM; this is a default set by
	// vagrant and may have to be adjusted if vagrant configuration is changed
	VM_LOCAL_IP   = "10.0.2.15"
	LOCALHOST_IP  = "127.0.0.1"
	HTTP_PROTOCOL = "http"
	HB_PERIOD, _  = goconf.AppConf.Int("heartbeat.period")
	HB_LIMIT, _   = goconf.AppConf.Int("heartbeat.limit")

	GRAFANA_URL   = goconf.AppConf.String("grafana.url")
	TUTORIALS_URL = goconf.AppConf.String("tutorials.url")

	// Image building service information
	IMG_BUILD_ADDRESS        = goconf.AppConf.String("img_builder.address")
	IMG_BUILD_SECRET_TOKEN   = goconf.AppConf.String("img_builder.secret_token")
	IMG_BUILD_BUILD_DELAY, _ = goconf.AppConf.Int64("img_builder.build_delay")
)

func init() {
	sp := goconf.AppConf.DefaultInt("br_bind_start_port", 50000)
	BR_START_PORT = uint16(sp) // Ports are only 16 bits
	sp = goconf.AppConf.DefaultInt("br_internal_start_port", 31046)
	BR_INTERNAL_START_PORT = uint16(sp) // Ports are only 16 bits
	if HTTP_ENABLE_HTTPS {
		HTTP_PROTOCOL = "https"
	}
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
		vi, err := strconv.Atoi(v)
		if err != nil {
			fmt.Println("Error parsing section signing_ases:", err)
			os.Exit(1)
		}
		SIGNING_ASES[ki] = vi
	}
}

func MaxASes(isAdmin bool) int {
	if isAdmin {
		return ASES_PER_ADMIN
	}
	return ASES_PER_USER
}
