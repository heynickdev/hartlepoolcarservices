package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"hcs-full/database"
	"hcs-full/models"
	"hcs-full/utils"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgtype"
)

var templates *template.Template
var funcMap = template.FuncMap{
	"toJSON": func(v interface{}) (template.JS, error) {
		a, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return template.JS(a), nil
	},
}

func init() {
	files, err := filepath.Glob("templates/*.html")
	if err != nil {
		log.Fatal(err)
	}

	layouts, err := filepath.Glob("templates/layouts/*.html")
	if err != nil {
		log.Fatal(err)
	}

	files = append(files, layouts...)
	templates = template.Must(template.New("").Funcs(funcMap).ParseFiles(files...))
}

// RenderTemplate renders a full page template from the pre-parsed cache.
func RenderTemplate(w http.ResponseWriter, r *http.Request, tmplName string, data models.PageData) {
	// For every page render, check for a valid token to ensure the header is always correct.
	c, err := r.Cookie("token")
	if err == nil {
		claims, err := utils.ParseJWT(c.Value)
		if err == nil {
			data.IsAuthenticated = true
			// If the handler didn't already provide user data (e.g., for static pages),
			// fetch it so the header can display it.
			if data.User == nil {
				user, err := database.Queries.GetUserByID(context.Background(), pgtype.UUID{Bytes: claims.UserID, Valid: true})
				if err == nil {
					data.User = &user
				}
			}
		}
	}

	buf := new(bytes.Buffer)
	// Execute the specific template by its base name (e.g., "index.html") from the global cache.
	err = templates.ExecuteTemplate(buf, tmplName, data)
	if err != nil {
		log.Printf("Error executing template %s: %v", tmplName, err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
	buf.WriteTo(w)
}

// RenderPartialTemplate renders a single template file from the pre-parsed cache.
func RenderPartialTemplate(w http.ResponseWriter, r *http.Request, tmplName string, data interface{}) {
	err := templates.ExecuteTemplate(w, tmplName, data)
	if err != nil {
		log.Printf("Partial template execution error for %s: %v", tmplName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}


