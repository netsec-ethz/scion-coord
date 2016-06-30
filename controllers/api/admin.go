package api

import (
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"html/template"
	"net/http"
)

type AdminController struct {
	controllers.HTTPController
}

func (c *AdminController) Index(w http.ResponseWriter, r *http.Request) {

	// get the current user session if present.
	// if not then, abort
	_, userSession, err := middleware.GetUserSession(r)

	if err != nil || userSession == nil {
		http.Redirect(w, r, "/login", 302)
		return
	}

	t, err := template.ParseFiles("templates/admin.html")
	if err != nil {
		c.Error500(err, w, r)
		return
	}
	c.Render(t, nil, w, r)
}
