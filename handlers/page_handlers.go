package handlers

import (
	"hcs-full/models"
	"net/http"
)

type pageData struct {
	Title string
	MetaDescription string
	CanonicalURL string
}

func HomePage(w http.ResponseWriter, r *http.Request) {
	data := pageData{
		Title: "Home",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL: "/",
	}
	RenderTemplate(w, r, "index.html", models.PageData{Title: data.Title, MetaDescription: data.MetaDescription, CanonicalURL: data.CanonicalURL})
}

func AboutPage(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, r, "about.html", models.PageData{Title: "About Us"})
}

func ServicesPage(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, r, "services.html", models.PageData{Title: "Our Services"})
}

func ContactPage(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, r, "contact.html", models.PageData{Title: "Contact Us"})
}

func PrivacyPolicyPage(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, r, "privacy_policy.html", models.PageData{Title: "Privacy Policy"})
}

func TermsAndConditionsPage(w http.ResponseWriter, r *http.Request) {
	RenderTemplate(w, r, "terms_and_conditions.html", models.PageData{Title: "Terms & Conditions"})
}


