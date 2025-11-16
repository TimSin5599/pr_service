package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

type Postgres struct {
	db *pgxpool.Pool
}

func New(connString string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse config error: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("connection error: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping error: %w", err)
	}

	return &Postgres{db: pool}, nil
}

func NewWithPool(pool *pgxpool.Pool) (*Postgres, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool cannot be nil")
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("pool ping error: %w", err)
	}

	return &Postgres{db: pool}, nil
}

func (p *Postgres) Close() {
	if p.db != nil {
		p.db.Close()
	}
}

type UserRepo struct {
	db *pgxpool.Pool
}

func (p *Postgres) UserRepo() *UserRepo {
	return &UserRepo{db: p.db}
}

func (r *UserRepo) Create(ctx context.Context, u entity.User) error {
	query := `
		INSERT INTO users (user_id, username, team_name, is_active)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			username = EXCLUDED.username,
			team_name = EXCLUDED.team_name,
			is_active = EXCLUDED.is_active
	`
	_, err := r.db.Exec(ctx, query, u.UserID, u.Username, u.TeamName, u.IsActive)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (entity.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active
		FROM users WHERE user_id = $1
	`
	var u entity.User
	
	err := r.db.QueryRow(ctx, query, id).Scan(
		&u.UserID, &u.Username, &u.TeamName, &u.IsActive,
	)
	if err == pgx.ErrNoRows {
		return entity.User{}, ErrNotFound
	}
	if err != nil {
		return entity.User{}, err
	}
	
	return u, nil
}

func (r *UserRepo) Update(ctx context.Context, u entity.User) error {
	query := `
		UPDATE users 
		SET username = $1, team_name = $2, is_active = $3
		WHERE user_id = $4
	`
	result, err := r.db.Exec(ctx, query, u.Username, u.TeamName, u.IsActive, u.UserID)
	if err != nil {
		return err
	}
	
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *UserRepo) ListByTeam(ctx context.Context, teamName string) ([]entity.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active
		FROM users WHERE team_name = $1
	`
	rows, err := r.db.Query(ctx, query, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []entity.User
	for rows.Next() {
		var u entity.User
		
		if err := rows.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

func (r *UserRepo) ListAll(ctx context.Context) ([]entity.User, error) {
	query := `
		SELECT user_id, username, team_name, is_active
		FROM users
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []entity.User
	for rows.Next() {
		var u entity.User
		
		if err := rows.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	return users, nil
}

type TeamRepo struct {
	db *pgxpool.Pool
}

func (p *Postgres) TeamRepo() *TeamRepo {
	return &TeamRepo{db: p.db}
}

func (r *TeamRepo) Create(ctx context.Context, t entity.Team) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)", t.TeamName).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyExists
	}

	_, err = tx.Exec(ctx, "INSERT INTO teams (team_name) VALUES ($1)", t.TeamName)
	if err != nil {
		return err
	}

	for _, member := range t.Members {
		_, err = tx.Exec(ctx, `
			INSERT INTO users (user_id, username, team_name, is_active)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET
				username = EXCLUDED.username,
				team_name = EXCLUDED.team_name,
				is_active = EXCLUDED.is_active
		`, member.UserID, member.Username, t.TeamName, member.IsActive)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *TeamRepo) GetByName(ctx context.Context, name string) (entity.Team, error) {
	query := `
		SELECT user_id, username, is_active
		FROM users 
		WHERE team_name = $1
		ORDER BY user_id
	`
	rows, err := r.db.Query(ctx, query, name)
	if err != nil {
		return entity.Team{}, err
	}
	defer rows.Close()

	var team entity.Team
	team.TeamName = name

	for rows.Next() {
		var member entity.TeamMember
		if err := rows.Scan(&member.UserID, &member.Username, &member.IsActive); err != nil {
			return entity.Team{}, err
		}
		team.Members = append(team.Members, member)
	}

	if len(team.Members) == 0 {
		return entity.Team{}, ErrNotFound
	}

	return team, nil
}

func (r *TeamRepo) ListAll(ctx context.Context) ([]entity.Team, error) {
	query := `
		SELECT DISTINCT team_name 
		FROM users 
		WHERE team_name IS NOT NULL AND team_name != ''
		ORDER BY team_name
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []entity.Team
	for rows.Next() {
		var teamName string
		if err := rows.Scan(&teamName); err != nil {
			return nil, err
		}

		team, err := r.GetByName(ctx, teamName)
		if err != nil {
			continue
		}
		teams = append(teams, team)
	}

	return teams, nil
}

type PRRepo struct {
	db *pgxpool.Pool
}

func (p *Postgres) PRRepo() *PRRepo {
	return &PRRepo{db: p.db}
}

func (r *PRRepo) Create(ctx context.Context, pr entity.PullRequest) error {
	query := `
		INSERT INTO pull_requests (
			pull_request_id, pull_request_name, author_id, status, 
			assigned_reviewers, created_at, merged_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	
	reviewersJSON, err := json.Marshal(pr.AssignedReviewers)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, string(pr.Status),
		reviewersJSON, pr.CreatedAt, pr.MergedAt,
	)
	
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return ErrAlreadyExists
		}
		return err
	}
	
	return nil
}

func (r *PRRepo) GetByID(ctx context.Context, id string) (entity.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status,
		       assigned_reviewers, created_at, merged_at
		FROM pull_requests WHERE pull_request_id = $1
	`
	
	var pr entity.PullRequest
	var status string
	var reviewersJSON []byte
	var mergedAt sql.NullTime
	
	err := r.db.QueryRow(ctx, query, id).Scan(
		&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &status,
		&reviewersJSON, &pr.CreatedAt, &mergedAt,
	)
	
	if err == pgx.ErrNoRows {
		return entity.PullRequest{}, ErrNotFound
	}
	if err != nil {
		return entity.PullRequest{}, err
	}
	
	pr.Status = entity.PRStatus(status)
	
	if err := json.Unmarshal(reviewersJSON, &pr.AssignedReviewers); err != nil {
		return entity.PullRequest{}, err
	}
	
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}
	
	return pr, nil
}

func (r *PRRepo) Update(ctx context.Context, pr entity.PullRequest) error {
	query := `
		UPDATE pull_requests 
		SET pull_request_name = $1, author_id = $2, status = $3,
		    assigned_reviewers = $4, merged_at = $5
		WHERE pull_request_id = $6
	`
	
	reviewersJSON, err := json.Marshal(pr.AssignedReviewers)
	if err != nil {
		return err
	}
	
	result, err := r.db.Exec(ctx, query,
		pr.PullRequestName, pr.AuthorID, string(pr.Status),
		reviewersJSON, pr.MergedAt, pr.PullRequestID,
	)
	
	if err != nil {
		return err
	}
	
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	
	return nil
}

func (r *PRRepo) ListByReviewer(ctx context.Context, reviewerID string) ([]entity.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status,
		       assigned_reviewers, created_at, merged_at
		FROM pull_requests 
		WHERE assigned_reviewers @> $1::jsonb
		ORDER BY created_at DESC
	`
	
	reviewerJSON, err := json.Marshal([]string{reviewerID})
	if err != nil {
		return nil, err
	}
	
	rows, err := r.db.Query(ctx, query, reviewerJSON)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []entity.PullRequest
	for rows.Next() {
		var pr entity.PullRequest
		var status string
		var reviewersJSON []byte
		var mergedAt sql.NullTime
		
		if err := rows.Scan(
			&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &status,
			&reviewersJSON, &pr.CreatedAt, &mergedAt,
		); err != nil {
			return nil, err
		}
		
		pr.Status = entity.PRStatus(status)
		
		if err := json.Unmarshal(reviewersJSON, &pr.AssignedReviewers); err != nil {
			return nil, err
		}
		
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}
		
		prs = append(prs, pr)
	}

	return prs, nil
}

func (r *PRRepo) ListAll(ctx context.Context) ([]entity.PullRequest, error) {
	query := `
		SELECT pull_request_id, pull_request_name, author_id, status,
		       assigned_reviewers, created_at, merged_at
		FROM pull_requests 
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []entity.PullRequest
	for rows.Next() {
		var pr entity.PullRequest
		var status string
		var reviewersJSON []byte
		var mergedAt sql.NullTime
		
		if err := rows.Scan(
			&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &status,
			&reviewersJSON, &pr.CreatedAt, &mergedAt,
		); err != nil {
			return nil, err
		}
		
		pr.Status = entity.PRStatus(status)
		
		if err := json.Unmarshal(reviewersJSON, &pr.AssignedReviewers); err != nil {
			return nil, err
		}
		
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}
		
		prs = append(prs, pr)
	}

	return prs, nil
}

var (
	_ usecase.UserRepo = (*UserRepo)(nil)
	_ usecase.TeamRepo = (*TeamRepo)(nil)
	_ usecase.PRRepo   = (*PRRepo)(nil)
)