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

var (
	HTTP_HOST                = goconf.AppConf.DefaultString("http.host", "127.0.0.1")
	HTTP_PORT                = goconf.AppConf.DefaultString("http.port", "8080")
	EMAIL_HOST               = goconf.AppConf.DefaultString("email.host", "127.0.0.1")
	EMAIL_PORT               = goconf.AppConf.DefaultInt("email.port", 25)
	EMAIL_FROM               = goconf.AppConf.DefaultString("email.from", "no-reply@coord.scionproto.net")
	SESSION_PATH             = goconf.AppConf.DefaultString("session.path", "tmp")
	SESSION_ENCRYPTION_KEY   = goconf.AppConf.DefaultString("session.encryption_key", "x290jdxmcam9q2dci:LWC92cqwop,011DMWCMWD")
	SESSION_VERIFICATION_KEY = goconf.AppConf.DefaultString("session.verification_key", "c23omc2o,pb45,-34l=12ms21odmx1;f230fm22fm")
	LOG_FILE                 = goconf.AppConf.DefaultString("log.file", "")
	DB_NAME                  = goconf.AppConf.DefaultString("db.name", "scion_coord_test")
	DB_HOST                  = goconf.AppConf.DefaultString("db.host", "127.0.0.1")
	DB_PORT                  = goconf.AppConf.DefaultString("db.port", "3306")
	DB_USER                  = goconf.AppConf.DefaultString("db.user", "root")
	DB_PASS                  = goconf.AppConf.DefaultString("db.pass", "")
	DB_MAX_CONNECTIONS       = goconf.AppConf.DefaultInt("db.max_connections", 15)
	DB_MAX_IDLE              = goconf.AppConf.DefaultInt("db.max_idle", 3)
)
