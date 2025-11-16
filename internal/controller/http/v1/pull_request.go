package v1

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/entity"
	usecase "github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

type PRHandler struct {
	uc    *usecase.PRUseCase
	users usecase.UserRepo
	teams usecase.TeamRepo
	prs   usecase.PRRepo
	l     logger.Interface
}

func NewHandler(uc *usecase.PRUseCase, userRepo usecase.UserRepo, teamRepo usecase.TeamRepo, prRepo usecase.PRRepo, l logger.Interface) *PRHandler {
	return &PRHandler{
		uc:   	uc,
		teams: 	teamRepo,
		users: 	userRepo,
		prs:   	prRepo,
		l:     	l,
	}
}

func (h *PRHandler) RegisterPRRoutes(router fiber.Router) {
	// Teams
	teamGroup := router.Group("/team")
	teamGroup.Post("/add", h.teamAdd)
	teamGroup.Get("/get", h.teamGet)

	// Users
	userGroup := router.Group("/users")
	userGroup.Post("/setIsActive", h.usersSetIsActive)
	userGroup.Get("/getReview", h.usersGetReview)
	userGroup.Post("/deactivateTeam", h.usersDeactivateTeam)

	// Pull Requests
	prGroup := router.Group("/pullRequest")
	prGroup.Post("/create", h.pullRequestCreate)
	prGroup.Post("/merge", h.pullRequestMerge)
	prGroup.Post("/reassign", h.pullRequestReassign)

	// Stats
	statsGroup := router.Group("/stats")
	statsGroup.Get("", h.getStats)
}

// teamAdd implements POST /team/add
func (h *PRHandler) teamAdd(c *fiber.Ctx) error {
	var t entity.Team
	if err := c.BodyParser(&t); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid body"}})
	}
	// check existing
	if _, err := h.teams.GetByName(c.Context(), t.TeamName); err == nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "TEAM_EXISTS", "message": "team_name already exists"}})
	}
	if err := h.teams.Create(c.Context(), t); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"team": t})
}

// teamGet implements GET /team/get?team_name=...
func (h *PRHandler) teamGet(c *fiber.Ctx) error {
	name := c.Query("team_name")
	if name == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "team_name required"}})
	}
	t, err := h.teams.GetByName(c.Context(), name)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "team not found"}})
	}
	return c.JSON(t)
}

// usersSetIsActive implements POST /users/setIsActive
func (h *PRHandler) usersSetIsActive(c *fiber.Ctx) error {
	var body struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid body"}})
	}
	u, err := h.users.GetByID(c.Context(), body.UserID)
	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "user not found"}})
	}
	u.IsActive = body.IsActive
	if err := h.users.Update(c.Context(), u); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
	}
	return c.JSON(fiber.Map{"user": u})
}

// usersGetReview implements GET /users/getReview?user_id=...
func (h *PRHandler) usersGetReview(c *fiber.Ctx) error {
	id := c.Query("user_id")
	if id == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "user_id required"}})
	}
	prs, err := h.prs.ListByReviewer(c.Context(), id)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
	}
	// build short representation
	short := make([]entity.PullRequestShort, 0, len(prs))
	for _, p := range prs {
		short = append(short, entity.PullRequestShort{
			PullRequestID:   p.PullRequestID,
			PullRequestName: p.PullRequestName,
			AuthorID:        p.AuthorID,
			Status:          p.Status,
		})
	}
	return c.JSON(fiber.Map{"user_id": id, "pull_requests": short})
}

// usersDeactivateTeam implements POST /users/deactivateTeam
func (h *PRHandler) usersDeactivateTeam(c *fiber.Ctx) error {
	var body struct {
		TeamName string `json:"team_name"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid body"}})
	}
	if body.TeamName == "" {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "team_name required"}})
	}
	if err := h.uc.DeactivateTeam(c.Context(), body.TeamName); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "team deactivated"})
}

// pullRequestCreate implements POST /pullRequest/create
func (h *PRHandler) pullRequestCreate(c *fiber.Ctx) error {
	var body struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid body"}})
	}
	pr, err := h.uc.CreatePR(c.Context(), body.PullRequestID, body.PullRequestName, body.AuthorID)
	if err != nil {
		switch err {
		case usecase.ErrNotFound:
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "author or team not found"}})
		case usecase.ErrPRExists:
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": fiber.Map{"code": "PR_EXISTS", "message": "PR id already exists"}})
		default:
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
		}
	}
	return c.Status(http.StatusCreated).JSON(fiber.Map{"pr": pr})
}

// pullRequestMerge implements POST /pullRequest/merge
func (h *PRHandler) pullRequestMerge(c *fiber.Ctx) error {
	var body struct {
		PullRequestID string `json:"pull_request_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid body"}})
	}
	pr, err := h.uc.MergePR(c.Context(), body.PullRequestID)
	if err != nil {
		if err == usecase.ErrNotFound {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "pr not found"}})
		}
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
	}
	return c.JSON(fiber.Map{"pr": pr})
}

// pullRequestReassign implements POST /pullRequest/reassign
func (h *PRHandler) pullRequestReassign(c *fiber.Ctx) error {
	var body struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": fiber.Map{"code": "BAD_REQUEST", "message": "invalid body"}})
	}
	pr, replacedBy, err := h.uc.ReassignReviewer(c.Context(), body.PullRequestID, body.OldUserID)
	if err != nil {
		switch err {
		case usecase.ErrNotFound:
			return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_FOUND", "message": "pr or user not found"}})
		case usecase.ErrPRMerged:
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": fiber.Map{"code": "PR_MERGED", "message": "cannot reassign on merged PR"}})
		case usecase.ErrNotAssigned:
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": fiber.Map{"code": "NOT_ASSIGNED", "message": "reviewer is not assigned to this PR"}})
		case usecase.ErrNoCandidate:
			return c.Status(http.StatusConflict).JSON(fiber.Map{"error": fiber.Map{"code": "NO_CANDIDATE", "message": "no active replacement candidate in team"}})
		default:
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
		}
	}
	return c.JSON(fiber.Map{"pr": pr, "replaced_by": replacedBy})
}

// getStats implements GET /stats
func (h *PRHandler) getStats(c *fiber.Ctx) error {
	stats, err := h.uc.GetStats(c.Context())
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": fiber.Map{"code": "INTERNAL", "message": err.Error()}})
	}
	return c.JSON(fiber.Map{"stats": stats})
}
