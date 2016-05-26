package api

import (
	"fmt"
	"github.com/netsec-ethz/scion-coord/controllers"
	"html"
	"log"
	"net/http"
)

type AdminController struct {
	controllers.HTTPController
}

func (c *AdminController) Index(w http.ResponseWriter, r *http.Request) {
	log.Println("AUTHORIZED")
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
}
