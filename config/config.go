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
	"os"
	"path/filepath"

	"github.com/sec51/goconf"
)

// Settings are specified in conf/development.conf
var (
	// address the service listens on
	HTTP_BIND_ADDRESS = goconf.AppConf.DefaultString("http.bind_address", "127.0.0.1")
	// port the service listens on
	HTTP_BIND_PORT = goconf.AppConf.DefaultInt("http.bind_port", 8080)
	// address from which the service is reachable from outside
	HTTP_HOST_ADDRESS        = goconf.AppConf.String("http.host_address")
	HTTP_ENABLE_HTTPS        = goconf.AppConf.DefaultBool("http.enable_https", false)
	EMAIL_PM_SERVER_TOKEN    = goconf.AppConf.String("email.pm_server_token")
	EMAIL_PM_ACCOUNT_TOKEN   = goconf.AppConf.String("email.pm_account_token")
	EMAIL_FROM               = goconf.AppConf.String("email.from")
	CAPTCHA_SECRET_KEY       = goconf.AppConf.String("captcha.secret_key")
	CAPTCHA_SITE_KEY         = goconf.AppConf.String("captcha.site_key")
	SESSION_PATH             = goconf.AppConf.String("session.path")
	SESSION_ENCRYPTION_KEY   = goconf.AppConf.String("session.encryption_key")
	SESSION_VERIFICATION_KEY = goconf.AppConf.String("session.verification_key")
	LOG_FILE                 = goconf.AppConf.String("log.file")
	PACKAGE_DIRECTORY        = goconf.AppConf.DefaultString("directory.package_directory",
		filepath.Join(os.Getenv("HOME"), "scionLabConfigs"))
	DB_NAME               = goconf.AppConf.String("db.name")
	DB_HOST               = goconf.AppConf.String("db.host")
	DB_PORT               = goconf.AppConf.DefaultInt("db.port", 3306)
	DB_USER               = goconf.AppConf.String("db.user")
	DB_PASS               = goconf.AppConf.String("db.pass")
	DB_MAX_CONNECTIONS    = goconf.AppConf.DefaultInt("db.max_connections", 15)
	DB_MAX_IDLE           = goconf.AppConf.DefaultInt("db.max_idle", 3)
	SERVER_IA             = goconf.AppConf.String("server.ia")
	SERVER_IP             = goconf.AppConf.String("server.ip")
	SERVER_START_PORT     = goconf.AppConf.DefaultInt("server.start_port", 50050)
	SERVER_VPN_IP         = goconf.AppConf.String("server.vpn.ip")
	SERVER_VPN_START_IP   = goconf.AppConf.String("server.vpn.start_ip")
	SERVER_VPN_END_IP     = goconf.AppConf.String("server.vpn.end_ip")
	SERVER_VPN_START_PORT = goconf.AppConf.DefaultInt("server.vpn.start_port", 50050)
)
