package models

import "time"

// Profile is the portfolio's identity block plus the four ordered lists that
// used to live in the Payload CMS (personalInfo, skills, experiences, projects,
// education). One request returns all of it: the homepage renders the whole
// set at once, so splitting it into five endpoints would only add round trips.
//
// JSON keys keep the camelCase the frontend already consumes (personalInfo,
// githubUrl, ...) so the switch from the CMS endpoint was a URL change, not a
// reshape of every component's props.
type Profile struct {
	PersonalInfo PersonalInfo `json:"personalInfo"`
	Skills       []Skill      `json:"skills"`
	Experiences  []Experience `json:"experiences"`
	Projects     []Project    `json:"projects"`
	Education    []Education  `json:"education"`
}

// PersonalInfo mirrors profile.personal_info — a single row (id = 1).
// about/bio carry both locales; the frontend picks by active locale, which is
// why they are two columns rather than a translation table.
type PersonalInfo struct {
	Fullname  string `json:"fullname"`
	Nickname  string `json:"nickname"`
	Title     string `json:"title"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Location  string `json:"location"`
	Github    string `json:"github"`
	Linkedin  string `json:"linkedin"`
	Instagram string `json:"instagram"`
	WhatsApp  string `json:"whatsApp"`
	Website   string `json:"website"`
	AboutID   string `json:"about_id"`
	AboutEN   string `json:"about_en"`
	BioID     string `json:"bio_id"`
	BioEN     string `json:"bio_en"`
}

type Skill struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Level    string `json:"level"`
}

type Experience struct {
	Company      string   `json:"company"`
	Position     string   `json:"position"`
	Duration     string   `json:"duration"`
	Description  []string `json:"description"`
	Technologies []string `json:"technologies"`
}

type Project struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Technologies []string `json:"technologies"`
	GithubURL    string   `json:"githubUrl"`
}

type Education struct {
	Institution string `json:"institution"`
	Degree      string `json:"degree"`
	Field       string `json:"field"`
	Duration    string `json:"duration"`
	GPA         string `json:"gpa"`
}

// ProfileResponse is what /api/v1/profile returns. UpdatedAt lets the frontend
// (and a human) tell when the data last changed without opening the database.
type ProfileResponse struct {
	Profile
	UpdatedAt time.Time `json:"updated_at"`
}
