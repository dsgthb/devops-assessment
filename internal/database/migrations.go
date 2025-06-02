package database

import (
	"database/sql"
	"fmt"
	"log"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          func(*sql.Tx) error
	Down        func(*sql.Tx) error
}

// GetMigrations returns all database migrations
func GetMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create initial schema with RBAC",
			Up:          migration001Up,
			Down:        migration001Down,
		},
	}
}

func migration001Up(tx *sql.Tx) error {
	queries := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id INT PRIMARY KEY AUTO_INCREMENT,
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			first_name VARCHAR(100),
			last_name VARCHAR(100),
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_users_email (email),
			INDEX idx_users_active (is_active)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Groups table
		`CREATE TABLE IF NOT EXISTS groups (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_groups_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Teams table
		`CREATE TABLE IF NOT EXISTS teams (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			group_id INT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE SET NULL,
			INDEX idx_teams_name (name),
			INDEX idx_teams_group (group_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Roles table
		`CREATE TABLE IF NOT EXISTS roles (
			id INT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(50) UNIQUE NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_roles_name (name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Permissions table
		`CREATE TABLE IF NOT EXISTS permissions (
			id INT PRIMARY KEY AUTO_INCREMENT,
			resource VARCHAR(100) NOT NULL,
			action VARCHAR(50) NOT NULL,
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY unique_permission (resource, action),
			INDEX idx_permissions_resource (resource)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Role permissions mapping
		`CREATE TABLE IF NOT EXISTS role_permissions (
			role_id INT NOT NULL,
			permission_id INT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (role_id, permission_id),
			FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
			FOREIGN KEY (permission_id) REFERENCES permissions(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// User groups association
		`CREATE TABLE IF NOT EXISTS user_groups (
			user_id INT NOT NULL,
			group_id INT NOT NULL,
			role_id INT NOT NULL,
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, group_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (group_id) REFERENCES groups(id) ON DELETE CASCADE,
			FOREIGN KEY (role_id) REFERENCES roles(id),
			INDEX idx_user_groups_user (user_id),
			INDEX idx_user_groups_group (group_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// User teams association
		`CREATE TABLE IF NOT EXISTS user_teams (
			user_id INT NOT NULL,
			team_id INT NOT NULL,
			role_id INT NOT NULL,
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, team_id),
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
			FOREIGN KEY (role_id) REFERENCES roles(id),
			INDEX idx_user_teams_user (user_id),
			INDEX idx_user_teams_team (team_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Assessments table
		`CREATE TABLE IF NOT EXISTS assessments (
			id INT PRIMARY KEY AUTO_INCREMENT,
			team_id INT NOT NULL,
			created_by INT NOT NULL,
			session_id VARCHAR(255),
			status ENUM('in_progress', 'completed') DEFAULT 'in_progress',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP NULL,
			FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
			FOREIGN KEY (created_by) REFERENCES users(id),
			INDEX idx_assessments_team (team_id),
			INDEX idx_assessments_creator (created_by),
			INDEX idx_assessments_status (status),
			INDEX idx_assessments_session (session_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Responses table
		`CREATE TABLE IF NOT EXISTS responses (
			id INT PRIMARY KEY AUTO_INCREMENT,
			assessment_id INT NOT NULL,
			question_id VARCHAR(20) NOT NULL,
			answer_ids TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (assessment_id) REFERENCES assessments(id) ON DELETE CASCADE,
			INDEX idx_responses_assessment (assessment_id),
			INDEX idx_responses_question (question_id),
			UNIQUE KEY unique_assessment_question (assessment_id, question_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Section scores table
		`CREATE TABLE IF NOT EXISTS section_scores (
			id INT PRIMARY KEY AUTO_INCREMENT,
			assessment_id INT NOT NULL,
			section_name VARCHAR(100) NOT NULL,
			score DECIMAL(5,2),
			max_score DECIMAL(5,2),
			percentage DECIMAL(5,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (assessment_id) REFERENCES assessments(id) ON DELETE CASCADE,
			INDEX idx_scores_assessment (assessment_id),
			UNIQUE KEY unique_assessment_section (assessment_id, section_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Audit logs table
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INT PRIMARY KEY AUTO_INCREMENT,
			user_id INT,
			action VARCHAR(100) NOT NULL,
			resource_type VARCHAR(50),
			resource_id INT,
			details JSON,
			ip_address VARCHAR(45),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
			INDEX idx_audit_user (user_id),
			INDEX idx_audit_action (action),
			INDEX idx_audit_resource (resource_type, resource_id),
			INDEX idx_audit_created (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// User sessions table
		`CREATE TABLE IF NOT EXISTS user_sessions (
			id INT PRIMARY KEY AUTO_INCREMENT,
			user_id INT NOT NULL,
			session_token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			INDEX idx_sessions_user (user_id),
			INDEX idx_sessions_token (session_token),
			INDEX idx_sessions_expires (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		// Schema version tracking
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			description VARCHAR(255),
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	// Execute table creation queries
	for _, query := range queries {
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %w", err)
		}
	}

	// Insert default roles
	roleQueries := []string{
		`INSERT INTO roles (name, description) VALUES 
			('admin', 'Full system access with ability to manage users, teams, and all assessments'),
			('editor', 'Can create and edit assessments, manage team assessments'),
			('viewer', 'Can view assessments and results only')`,
	}

	// Insert default permissions
	permissionQueries := []string{
		`INSERT INTO permissions (resource, action, description) VALUES 
			-- User permissions
			('user', 'create', 'Create new users'),
			('user', 'read', 'View user information'),
			('user', 'update', 'Update user information'),
			('user', 'delete', 'Delete users'),
			-- Team permissions
			('team', 'create', 'Create new teams'),
			('team', 'read', 'View team information'),
			('team', 'update', 'Update team information'),
			('team', 'delete', 'Delete teams'),
			-- Group permissions
			('group', 'create', 'Create new groups'),
			('group', 'read', 'View group information'),
			('group', 'update', 'Update group information'),
			('group', 'delete', 'Delete groups'),
			-- Assessment permissions
			('assessment', 'create', 'Create new assessments'),
			('assessment', 'read', 'View assessments'),
			('assessment', 'update', 'Update assessments'),
			('assessment', 'delete', 'Delete assessments'),
			-- Report permissions
			('report', 'read', 'View reports'),
			('report', 'export', 'Export reports to CSV/PDF'),
			-- System permissions
			('system', 'manage', 'Manage system settings'),
			('audit', 'read', 'View audit logs')`,
	}

	// Execute role and permission inserts
	for _, query := range roleQueries {
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("failed to insert roles: %w", err)
		}
	}

	for _, query := range permissionQueries {
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("failed to insert permissions: %w", err)
		}
	}

	// Map permissions to roles
	rolePermissionMapping := `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT r.id, p.id FROM roles r, permissions p
		WHERE 
			-- Admin gets all permissions
			(r.name = 'admin') OR
			-- Editor permissions
			(r.name = 'editor' AND (
				(p.resource = 'assessment' AND p.action IN ('create', 'read', 'update')) OR
				(p.resource = 'team' AND p.action = 'read') OR
				(p.resource = 'user' AND p.action = 'read') OR
				(p.resource = 'group' AND p.action = 'read') OR
				(p.resource = 'report' AND p.action IN ('read', 'export'))
			)) OR
			-- Viewer permissions
			(r.name = 'viewer' AND (
				(p.resource = 'assessment' AND p.action = 'read') OR
				(p.resource = 'team' AND p.action = 'read') OR
				(p.resource = 'report' AND p.action = 'read')
			))
	`

	if _, err := tx.Exec(rolePermissionMapping); err != nil {
		return fmt.Errorf("failed to map role permissions: %w", err)
	}

	// Record migration
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version, description) VALUES (?, ?)",
		1, "Create initial schema with RBAC",
	); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	log.Println("Migration 001: Initial schema with RBAC created successfully")
	return nil
}

func migration001Down(tx *sql.Tx) error {
	tables := []string{
		"schema_migrations",
		"user_sessions",
		"audit_logs",
		"section_scores",
		"responses",
		"assessments",
		"user_teams",
		"user_groups",
		"role_permissions",
		"permissions",
		"roles",
		"teams",
		"groups",
		"users",
	}

	for _, table := range tables {
		query := fmt.Sprintf("DROP TABLE IF EXISTS %s", table)
		if _, err := tx.Exec(query); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	log.Println("Migration 001: Rolled back successfully")
	return nil
}

// RunMigrations executes all pending migrations
func RunMigrations(db *sql.DB) error {
	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrations := GetMigrations()
	
	for _, migration := range migrations {
		applied, err := isMigrationApplied(db, migration.Version)
		if err != nil {
			return fmt.Errorf("failed to check migration status: %w", err)
		}
		
		if !applied {
			log.Printf("Running migration %d: %s", migration.Version, migration.Description)
			
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			
			if err := migration.Up(tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("migration %d failed: %w", migration.Version, err)
			}
			
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
			}
		}
	}
	
	return nil
}

func createMigrationsTable(db *sql.DB) error {
	query := `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INT PRIMARY KEY,
		description VARCHAR(255),
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	
	_, err := db.Exec(query)
	return err
}

func isMigrationApplied(db *sql.DB, version int) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM schema_migrations WHERE version = ?",
		version,
	).Scan(&count)
	
	if err != nil {
		return false, err
	}
	
	return count > 0, nil
}