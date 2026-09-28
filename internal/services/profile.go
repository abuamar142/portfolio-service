package services

import (
	"context"
	"fmt"

	"github.com/abuamar142/portfolio-service/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileService reads the identity block and the four ordered lists that
// replaced the Payload CMS. Read-only: this data changes a few times a year and
// is edited by SQL migration, so there is no write path to maintain.
type ProfileService struct {
	pool *pgxpool.Pool
}

func NewProfileService(pool *pgxpool.Pool) *ProfileService {
	return &ProfileService{pool: pool}
}

// Get loads everything the homepage needs. The five queries are independent,
// so they run in sequence over the pool — the data is tiny (54 rows total) and
// a transaction would add ceremony without buying consistency the page needs.
func (s *ProfileService) Get(ctx context.Context) (*models.ProfileResponse, error) {
	var out models.ProfileResponse

	if err := s.pool.QueryRow(ctx,
		`SELECT fullname, nickname, title, email, phone, location,
		        github, linkedin, instagram, whatsapp, website,
		        about_id, about_en, bio_id, bio_en, updated_at
		 FROM profile.personal_info
		 WHERE id = 1`,
	).Scan(
		&out.PersonalInfo.Fullname, &out.PersonalInfo.Nickname, &out.PersonalInfo.Title,
		&out.PersonalInfo.Email, &out.PersonalInfo.Phone, &out.PersonalInfo.Location,
		&out.PersonalInfo.Github, &out.PersonalInfo.Linkedin, &out.PersonalInfo.Instagram,
		&out.PersonalInfo.WhatsApp, &out.PersonalInfo.Website,
		&out.PersonalInfo.AboutID, &out.PersonalInfo.AboutEN,
		&out.PersonalInfo.BioID, &out.PersonalInfo.BioEN,
		&out.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("loading personal info: %w", err)
	}

	skills, err := s.skills(ctx)
	if err != nil {
		return nil, err
	}
	experiences, err := s.experiences(ctx)
	if err != nil {
		return nil, err
	}
	projects, err := s.projects(ctx)
	if err != nil {
		return nil, err
	}
	education, err := s.education(ctx)
	if err != nil {
		return nil, err
	}

	out.Skills = skills
	out.Experiences = experiences
	out.Projects = projects
	out.Education = education
	return &out, nil
}

// Each list is ordered by order_index: the CMS let the owner arrange entries,
// and that arrangement is part of the content, not incidental.
func (s *ProfileService) skills(ctx context.Context) ([]models.Skill, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT name, category, level
		 FROM profile.skills
		 ORDER BY order_index ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing skills: %w", err)
	}
	defer rows.Close()

	items := []models.Skill{} // non-nil: JSON [] instead of null
	for rows.Next() {
		var it models.Skill
		if err := rows.Scan(&it.Name, &it.Category, &it.Level); err != nil {
			return nil, fmt.Errorf("scanning skill: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *ProfileService) experiences(ctx context.Context) ([]models.Experience, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT company, position, duration, description, technologies
		 FROM profile.experiences
		 ORDER BY order_index ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing experiences: %w", err)
	}
	defer rows.Close()

	items := []models.Experience{}
	for rows.Next() {
		var it models.Experience
		if err := rows.Scan(&it.Company, &it.Position, &it.Duration,
			&it.Description, &it.Technologies); err != nil {
			return nil, fmt.Errorf("scanning experience: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *ProfileService) projects(ctx context.Context) ([]models.Project, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT title, description, technologies, github_url
		 FROM profile.projects
		 ORDER BY order_index ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	items := []models.Project{}
	for rows.Next() {
		var it models.Project
		if err := rows.Scan(&it.Title, &it.Description, &it.Technologies, &it.GithubURL); err != nil {
			return nil, fmt.Errorf("scanning project: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (s *ProfileService) education(ctx context.Context) ([]models.Education, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT institution, degree, field, duration, gpa
		 FROM profile.education
		 ORDER BY order_index ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("listing education: %w", err)
	}
	defer rows.Close()

	items := []models.Education{}
	for rows.Next() {
		var it models.Education
		if err := rows.Scan(&it.Institution, &it.Degree, &it.Field, &it.Duration, &it.GPA); err != nil {
			return nil, fmt.Errorf("scanning education: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
