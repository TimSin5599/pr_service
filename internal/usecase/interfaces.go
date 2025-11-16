package usecase

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
)

type PRRepo interface {
	Create(ctx context.Context, p entity.PullRequest) error
	GetByID(ctx context.Context, id string) (entity.PullRequest, error)
	Update(ctx context.Context, p entity.PullRequest) error
	ListByReviewer(ctx context.Context, reviewerID string) ([]entity.PullRequest, error)
	ListAll(ctx context.Context) ([]entity.PullRequest, error)
}

type UserRepo interface {
	Create(ctx context.Context, u entity.User) error
	GetByID(ctx context.Context, id string) (entity.User, error)
	Update(ctx context.Context, u entity.User) error
	ListByTeam(ctx context.Context, teamName string) ([]entity.User, error)
	ListAll(ctx context.Context) ([]entity.User, error)
}

type TeamRepo interface {
	Create(ctx context.Context, t entity.Team) error
	GetByName(ctx context.Context, name string) (entity.Team, error)
	ListAll(ctx context.Context) ([]entity.Team, error)
}
