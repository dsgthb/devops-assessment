-- DevOps Assessment Database Initialization Script
-- This script creates initial data for testing and demonstration

-- Ensure we're using the correct database
USE devops_assessment;

-- Insert a default organization/group if needed
INSERT INTO groups (id, name, description) VALUES 
(1, 'Default Organization', 'Default organization for initial setup')
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- Insert a default team
INSERT INTO teams (id, name, description, group_id) VALUES 
(1, 'Demo Team', 'Demo team for testing the assessment', 1)
ON DUPLICATE KEY UPDATE name = VALUES(name);

-- Create a demo user (password: demo123)
-- Note: This is for demonstration only. In production, users should be created through the application
INSERT INTO users (id, email, password_hash, first_name, last_name, is_active) VALUES 
(2, 'demo@example.com', '$2a$10$YourHashedPasswordHere', 'Demo', 'User', true)
ON DUPLICATE KEY UPDATE email = VALUES(email);

-- Assign demo user to demo team with editor role
INSERT INTO user_teams (user_id, team_id, role_id) VALUES 
(2, 1, 2) -- Editor role
ON DUPLICATE KEY UPDATE role_id = VALUES(role_id);

-- Grant necessary permissions for the application user
GRANT ALL PRIVILEGES ON devops_assessment.* TO 'devops'@'%';
FLUSH PRIVILEGES;