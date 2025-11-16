INSERT INTO teams (team_name) VALUES 
('backend'),
('frontend')
ON CONFLICT (team_name) DO NOTHING;

INSERT INTO users (user_id, username, team_name, is_active) VALUES
('u1', 'Alice', 'backend', true),
('u2', 'Bob', 'backend', true),
('u3', 'Charlie', 'backend', false),
('u4', 'David', 'frontend', true),
('u5', 'Eve', 'frontend', true),
('u6', 'Gleb', 'backend', true)
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, assigned_reviewers) VALUES
('pr-1001', 'Add search', 'u1', 'OPEN', '["u2", "u5"]'),
('pr-1002', 'Improve UI', 'u4', 'OPEN', '["u5"]'),
('pr-1003', 'Fix DB', 'u1', 'MERGED', '["u2"]')
ON CONFLICT (pull_request_id) DO NOTHING;