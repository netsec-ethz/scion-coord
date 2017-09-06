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
	"github.com/sec51/goconf"
)

// Settings are specified in conf/development.conf
var (
	HTTP_BIND_ADDRESS        = goconf.AppConf.String("http.bind_address") // address the service listens on
	HTTP_BIND_PORT, _        = goconf.AppConf.Int("http.bind_port")       // port the service listens on
	HTTP_HOST_ADDRESS        = goconf.AppConf.String("http.host_address") // address from which the service is reachable from outside
	EMAIL_PM_SERVER_TOKEN    = goconf.AppConf.String("email.pm_server_token")
	EMAIL_PM_ACCOUNT_TOKEN   = goconf.AppConf.String("email.pm_account_token")
	EMAIL_FROM               = goconf.AppConf.String("email.from")
	CAPTCHA_SECRET_KEY       = goconf.AppConf.String("captcha.secret_key")
	CAPTCHA_SITE_KEY         = goconf.AppConf.String("captcha.site_key")
	SESSION_PATH             = goconf.AppConf.String("session.path")
	SESSION_ENCRYPTION_KEY   = goconf.AppConf.String("session.encryption_key")
	SESSION_VERIFICATION_KEY = goconf.AppConf.String("session.verification_key")
	LOG_FILE                 = goconf.AppConf.String("log.file")
	PACKAGE_DIRECTORY        = goconf.AppConf.String("directory.package_directory")
	DB_NAME                  = goconf.AppConf.String("db.name")
	DB_HOST                  = goconf.AppConf.String("db.host")
	DB_PORT, _               = goconf.AppConf.Int("db.port")
	DB_USER                  = goconf.AppConf.String("db.user")
	DB_PASS                  = goconf.AppConf.String("db.pass")
	DB_MAX_CONNECTIONS, _    = goconf.AppConf.Int("db.max_connections")
	DB_MAX_IDLE, _           = goconf.AppConf.Int("db.max_idle")
	SERVER_IA                = goconf.AppConf.String("server.ia")
	SERVER_IP                = goconf.AppConf.String("server.ip")
	SERVER_START_PORT, _     = goconf.AppConf.Int("server.start_port")
	SERVER_VPN_IP            = goconf.AppConf.String("server.vpn.ip")
	SERVER_VPN_START_IP      = goconf.AppConf.String("server.vpn.start_ip")
	SERVER_VPN_START_PORT, _ = goconf.AppConf.Int("server.vpn.start_port")
)
