package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrPRExists    = errors.New("PR exists")
	ErrPRMerged    = errors.New("PR_MERGED")
	ErrNotAssigned = errors.New("NOT_ASSIGNED")
	ErrNoCandidate = errors.New("NO_CANDIDATE")
)

type PRUseCase struct {
	prRepo   PRRepo
	userRepo UserRepo
	teamRepo TeamRepo
}

func NewPRUseCase(prRepo PRRepo, userRepo UserRepo, teamRepo TeamRepo) *PRUseCase {
	return &PRUseCase{
		prRepo:   prRepo,
		userRepo: userRepo,
		teamRepo: teamRepo,
	}
}

func (uc *PRUseCase) CreatePR(ctx context.Context, prID, prName, authorID string) (entity.PullRequest, error) {
	existing, err := uc.prRepo.GetByID(ctx, prID)
	if err == nil && existing.PullRequestID != "" {
		return entity.PullRequest{}, ErrPRExists
	}

	author, err := uc.userRepo.GetByID(ctx, authorID)
	if err != nil {
		return entity.PullRequest{}, ErrNotFound
	}

	teamMembers, err := uc.userRepo.ListByTeam(ctx, author.TeamName)
	if err != nil {
		return entity.PullRequest{}, ErrNotFound
	}

	var reviewers []string
	for _, member := range teamMembers {
		if member.UserID != authorID && member.IsActive && len(reviewers) < 2 {
			reviewers = append(reviewers, member.UserID)
		}
	}

	pr := entity.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            entity.PRStatusOpen,
		AssignedReviewers: reviewers,
		CreatedAt:         time.Now(),
	}

	err = uc.prRepo.Create(ctx, pr)
	if err != nil {
		return entity.PullRequest{}, err
	}

	return pr, nil
}

func (uc *PRUseCase) MergePR(ctx context.Context, prID string) (entity.PullRequest, error) {
	pr, err := uc.prRepo.GetByID(ctx, prID)
	if err != nil {
		return entity.PullRequest{}, ErrNotFound
	}

	if pr.Status == entity.PRStatusMerged {
		return pr, nil
	}

	now := time.Now()
	pr.Status = entity.PRStatusMerged
	pr.MergedAt = &now

	err = uc.prRepo.Update(ctx, pr)
	if err != nil {
		return entity.PullRequest{}, err
	}

	return pr, nil
}

func (uc *PRUseCase) ReassignReviewer(ctx context.Context, prID, oldUserID string) (entity.PullRequest, string, error) {
	pr, err := uc.prRepo.GetByID(ctx, prID)
	if err != nil {
		return entity.PullRequest{}, "", ErrNotFound
	}

	if pr.Status == entity.PRStatusMerged {
		return entity.PullRequest{}, "", ErrPRMerged
	}

	found := false
	for i, reviewer := range pr.AssignedReviewers {
		if reviewer == oldUserID {
			found = true
			pr.AssignedReviewers = append(pr.AssignedReviewers[:i], pr.AssignedReviewers[i+1:]...)
			break
		}
	}
	if !found {
		return entity.PullRequest{}, "", ErrNotAssigned
	}

	author, err := uc.userRepo.GetByID(ctx, pr.AuthorID)
	if err != nil {
		return entity.PullRequest{}, "", ErrNotFound
	}

	teamMembers, err := uc.userRepo.ListByTeam(ctx, author.TeamName)
	if err != nil {
		return entity.PullRequest{}, "", ErrNotFound
	}

	var newReviewerID string
	for _, member := range teamMembers {
		if member.UserID != pr.AuthorID &&
			member.IsActive &&
			!contains(pr.AssignedReviewers, member.UserID) &&
			member.UserID != oldUserID {
			newReviewerID = member.UserID
			break
		}
	}

	if newReviewerID == "" {
		return entity.PullRequest{}, "", ErrNoCandidate
	}

	pr.AssignedReviewers = append(pr.AssignedReviewers, newReviewerID)

	err = uc.prRepo.Update(ctx, pr)
	if err != nil {
		return entity.PullRequest{}, "", err
	}

	return pr, newReviewerID, nil
}

func (uc *PRUseCase) DeactivateTeam(ctx context.Context, teamName string) error {
	users, err := uc.userRepo.ListByTeam(ctx, teamName)
	if err != nil {
		return err
	}

	for _, user := range users {
		user.IsActive = false
		if err := uc.userRepo.Update(ctx, user); err != nil {
			return err
		}
	}

	return nil
}

func (uc *PRUseCase) GetStats(ctx context.Context) (map[string]interface{}, error) {
	prs, err := uc.prRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	users, err := uc.userRepo.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_prs":         len(prs),
		"total_users":       len(users),
		"open_prs":          0,
		"merged_prs":        0,
		"active_users":      0,
		"average_reviewers": 0.0,
	}

	totalReviewers := 0
	for _, pr := range prs {
		if pr.Status == entity.PRStatusOpen {
			stats["open_prs"] = stats["open_prs"].(int) + 1
		} else if pr.Status == entity.PRStatusMerged {
			stats["merged_prs"] = stats["merged_prs"].(int) + 1
		}
		totalReviewers += len(pr.AssignedReviewers)
	}

	for _, user := range users {
		if user.IsActive {
			stats["active_users"] = stats["active_users"].(int) + 1
		}
	}

	if len(prs) > 0 {
		stats["average_reviewers"] = float64(totalReviewers) / float64(len(prs))
	}

	return stats, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
