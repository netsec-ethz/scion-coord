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

package middleware

import (
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/netsec-ethz/scion-coord/config"
)

func LoggingHandler(next http.Handler) http.Handler {
	// Log to a file
	if config.LOG_FILE != "" {
		logFile, err := os.OpenFile(config.LOG_FILE, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			// we can panic here, because it means there is a wrong config.
			panic(err)
		}
		return handlers.LoggingHandler(logFile, next)
	}

	// log to console
	return handlers.LoggingHandler(os.Stdout, next)
}
