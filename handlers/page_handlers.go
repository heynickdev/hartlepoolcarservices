package handlers

import (
	"hcs-full/models"
	"net/http"
)

func HomePage(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{
		Title:           "Home",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL:    "/",
	}
	RenderTemplate(w, r, "index.html", data)
}

func AboutPage(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{
		Title:           "About",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL:    "/about",
	}
	RenderTemplate(w, r, "about.html", data)
}

func ServicesPage(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{
		Title:           "Services",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL:    "/services",
	}
	RenderTemplate(w, r, "services.html", data)
}

func ContactPage(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{
		Title:           "Contact",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL:    "/contact",
	}
	RenderTemplate(w, r, "contact.html", data)
}

func PrivacyPolicyPage(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{
		Title:           "Privacy Policy",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL:    "/privacy-policy",
	}
	RenderTemplate(w, r, "privacy_policy.html", data)
}

func TermsAndConditionsPage(w http.ResponseWriter, r *http.Request) {
	data := models.PageData{
		Title:           "Terms & Conditions",
		MetaDescription: "Hartlepoepool Car Services. Premium automotive services in Hartlepool. Quality repairs, diagnostics, and maintenance for all vehicle makes and models.",
		CanonicalURL:    "/terms-and-conditions",
	}
	RenderTemplate(w, r, "terms_and_conditions.html", data)
}
