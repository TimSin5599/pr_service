package response

const (
	ErrorCodeTeamExists  = "TEAM_EXISTS"
	ErrorCodePRExists    = "PR_EXISTS"
	ErrorCodePRMerged    = "PR_MERGED"
	ErrorCodeNotAssigned = "NOT_ASSIGNED"
	ErrorCodeNoCandidate = "NO_CANDIDATE"
	ErrorCodeNotFound    = "NOT_FOUND"
)

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
