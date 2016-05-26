package middleware

import (
	"github.com/gorilla/handlers"
	"github.com/netsec-ethz/scion-coord/config"
	"net/http"
	"os"
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
