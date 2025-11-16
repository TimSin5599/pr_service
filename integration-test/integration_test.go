package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	// Base settings
	host     = "localhost"
	attempts = 20

	// Attempts connection
	httpURL        = "http://" + host + ":8080"
	healthPath     = httpURL + "/healthz"
	requestTimeout = 5 * time.Second

	// HTTP REST
	basePathV1 = httpURL + "/v1"
)

var errHealthCheck = fmt.Errorf("url %s is not available", healthPath)

func doWebRequestWithTimeout(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

func getHealthCheck(url string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)

	defer cancel()

	resp, err := doWebRequestWithTimeout(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return -1, err
	}

	defer resp.Body.Close()

	return resp.StatusCode, nil
}

func healthCheck(attempts int) error {
	for attempts > 0 {
		statusCode, err := getHealthCheck(healthPath)
		if err == nil && statusCode == http.StatusOK {
			return nil
		}
		time.Sleep(time.Second)
		attempts--
	}
	return errHealthCheck
}

func TestMain(m *testing.M) {
	err := healthCheck(attempts)
	if err != nil {
		log.Fatalf("Integration tests: httpURL %s is not available: %s", httpURL, err)
	}

	log.Printf("Integration tests: httpURL %s is available", httpURL)

	code := m.Run()
	os.Exit(code)
}

func TestE2EFlow(t *testing.T) {
	t.Log("E2E flow test...")

	t.Log("Step 1: Creating team 'backend4'...")
	teamBody := `{"team_name": "backend4", "members": [
		{"user_id": "u1", "username": "Alice", "is_active": true},
		{"user_id": "u2", "username": "Bob", "is_active": true},
		{"user_id": "u3", "username": "Charlie", "is_active": true},
		{"user_id": "u9", "username": "Donald", "is_active": true}
	]}`
	doRequest(t, "POST", basePathV1+"/team/add", teamBody, 201)
	t.Log("Team 'backend4' created successfully")

	t.Log("Step 2: Getting team 'backend4' info...")
	doRequest(t, "GET", basePathV1+"/team/get?team_name=backend4", "", 200)
	t.Log("Team info retrieved successfully")

	t.Log("Step 3: Creating first PR...")
	prBody := `{"pull_request_id":"pr-1024","pull_request_name":"Test PR","author_id":"u1"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/create", prBody, 201)
	t.Log("First PR created successfully")

	t.Log("👥 Step 4: Checking assigned reviews...")
	doRequest(t, "GET", basePathV1+"/users/getReview?user_id=u2", "", 200)
	t.Log("Reviews for u2 checked successfully")
	doRequest(t, "GET", basePathV1+"/users/getReview?user_id=u3", "", 200)
	t.Log("Reviews for u3 checked successfully")

	t.Log("Step 5: Reassigning reviewer...")
	reassignBody := `{"pull_request_id":"pr-1024","old_user_id":"u2"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/reassign", reassignBody, 200)
	t.Log("Reviewer reassigned successfully")

	t.Log("Step 6: Merging first PR...")
	mergeBody := `{"pull_request_id":"pr-1024"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/merge", mergeBody, 200)
	t.Log("First PR merged successfully")

	t.Log("Step 7: Testing idempotent merge...")
	doRequest(t, "POST", basePathV1+"/pullRequest/merge", mergeBody, 200)
	t.Log("Idempotent merge verified successfully")

	t.Log("Step 8: Deactivating user u3...")
	setInactive := `{"user_id":"u3","is_active":false}`
	doRequest(t, "POST", basePathV1+"/users/setIsActive", setInactive, 200)
	t.Log("User u3 deactivated successfully")

	t.Log("Step 9: Creating second PR...")
	prBody2 := `{"pull_request_id":"pr-1025","pull_request_name":"Test PR2","author_id":"u1"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/create", prBody2, 201)
	t.Log("Second PR created successfully")

	t.Log("Step 10: Testing PR creation with non-existent author...")
	badPR := `{"pull_request_id":"pr-1026","pull_request_name":"Bad PR","author_id":"not-exist"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/create", badPR, 404)
	t.Log("Properly handled non-existent author case")

	t.Log("Step 11: Getting system stats...")
	doRequest(t, "GET", basePathV1+"/stats", "", 200)
	t.Log("Stats retrieved successfully")

	t.Log("Step 12: Health check...")
	doRequest(t, "GET", httpURL+"/healthz", "", 200)
	t.Log("Health check passed")

	t.Log("All E2E tests completed successfully!")
}

func TestAdditionalScenarios(t *testing.T) {
	t.Log("Starting additional scenarios test...")

	t.Log("Testing duplicate team creation...")
	teamBody := `{"team_name": "duplicate-team", "members": [
		{"user_id": "duplicate-user", "username": "Duplicate User", "is_active": true}
	]}`
	doRequest(t, "POST", basePathV1+"/team/add", teamBody, 201)
	t.Log("First team creation successful")
	
	doRequest(t, "POST", basePathV1+"/team/add", teamBody, 400)
	t.Log("Duplicate team creation properly rejected")

	t.Log("🔍 Testing non-existent team...")
	doRequest(t, "GET", basePathV1+"/team/get?team_name=nonexistent", "", 404)
	t.Log("Non-existent team properly handled")

	t.Log("👥 Testing team deactivation...")
	deactivateBody := `{"team_name": "duplicate-team"}`
	doRequest(t, "POST", basePathV1+"/users/deactivateTeam", deactivateBody, 200)
	t.Log("Team deactivation successful")

	t.Log("Additional scenarios completed successfully!")
}

func doRequest(t *testing.T, method, url, body string, wantStatus int) *http.Response {
	req, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("Request creation error: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request error: %v", err)
	}
	
	if resp.StatusCode != wantStatus {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("Unexpected status: got %d, want %d, body: %s", resp.StatusCode, wantStatus, string(b))
	}
	
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if len(b) > 0 {
			t.Logf("📨 Response: %s", string(b))
		}
		resp.Body = io.NopCloser(bytes.NewBuffer(b))
	}
	
	return resp
}

func TestEdgeCases(t *testing.T) {
	t.Log("Starting edge cases test...")

	t.Log("Testing reassignment on merged PR...")
	prBody := `{"pull_request_id":"edge-pr-1","pull_request_name":"Edge PR","author_id":"u1"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/create", prBody, 201)
	t.Log("Edge PR created")

	mergeBody := `{"pull_request_id":"edge-pr-1"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/merge", mergeBody, 200)
	t.Log("Edge PR merged")

	reassignBody := `{"pull_request_id":"edge-pr-1","old_user_id":"u2"}`
	doRequest(t, "POST", basePathV1+"/pullRequest/reassign", reassignBody, 409)
	t.Log("Reassignment on merged PR properly rejected")

	t.Log("Testing setIsActive with non-existent user...")
	badUserBody := `{"user_id":"nonexistent-user","is_active":false}`
	doRequest(t, "POST", basePathV1+"/users/setIsActive", badUserBody, 404)
	t.Log("Non-existent user properly handled")

	t.Log("Edge cases completed successfully!")
}