# DevOps Maturity Assessment

A comprehensive web application for assessing DevOps maturity, built with Go, MySQL, and modern web technologies. This application helps teams understand their current DevOps capabilities and provides curated resources for improvement.

## Features

- **Multi-tenant Support**: Organizations, groups, and teams with RBAC
- **Role-Based Access Control**: Admin, Editor, and Viewer roles
- **Persistent Storage**: MySQL database for assessments and results
- **Interactive Survey**: 7 sections covering key DevOps areas
- **Visual Results**: Radar charts showing maturity levels
- **Resource Library**: Curated learning resources for each area
- **Export Functionality**: CSV export of assessment results
- **Audit Trail**: Complete logging of user actions
- **Responsive Design**: Works on desktop and mobile devices

## Technology Stack

- **Backend**: Go (Golang) with Gin framework
- **Database**: MySQL 8.0
- **Frontend**: HTML5, Bootstrap 4, jQuery, Chart.js
- **Authentication**: Session-based with secure tokens
- **Containerization**: Docker and Docker Compose

## Project Structure

```
devops-assessment/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── auth/
│   │   ├── authentication.go    # Authentication service
│   │   └── middleware.go        # Auth middleware
│   ├── config/
│   │   └── config.go           # Configuration management
│   ├── database/
│   │   ├── mysql.go            # Database connection
│   │   └── migrations.go       # Database migrations
│   ├── handlers/
│   │   ├── auth_handler.go     # Authentication endpoints
│   │   ├── survey_handler.go   # Survey endpoints
│   │   ├── user_handler.go     # User management
│   │   ├── team_handler.go     # Team management
│   │   └── results_handler.go  # Results viewing
│   ├── models/
│   │   ├── user.go             # User model
│   │   ├── team.go             # Team and Group models
│   │   ├── role.go             # RBAC models
│   │   ├── assessment.go       # Assessment model
│   │   └── question.go         # Question model
│   └── services/
│       └── survey_service.go   # Survey business logic
├── web/
│   ├── templates/
│   │   ├── base.html           # Base template
│   │   ├── login.html          # Login page
│   │   ├── dashboard.html      # User dashboard
│   │   ├── survey.html         # Survey questionnaire
│   │   ├── results.html        # Results display
│   │   ├── resources.html      # Resources library
│   │   ├── about.html          # About page
│   │   └── error.html          # Error pages
│   └── static/
│       ├── css/                # Stylesheets
│       ├── js/                 # JavaScript files
│       └── fontawesome/        # Icon fonts
├── configs/
│   ├── questions.json          # Survey questions
│   └── advice.json             # Improvement advice
├── scripts/
│   └── init.sql               # Database initialization
├── uploads/                    # User uploads directory
├── Dockerfile                  # Docker build file
├── docker-compose.yml          # Docker Compose config
├── go.mod                      # Go module file
├── go.sum                      # Go dependencies
├── .env.example               # Environment variables example
└── README.md                  # This file
```

## Installation

### Prerequisites

- Go 1.21 or higher
- MySQL 8.0 or higher
- Docker and Docker Compose (optional)

### Using Docker Compose (Recommended)

1. Clone the repository:
   ```bash
   git clone https://github.com/dsgthb/devops-assessment.git
   cd devops-assessment
   ```

2. Copy the environment file:
   ```bash
   cp .env.example .env
   ```

3. Update the `.env` file with your configuration

4. Start the application:
   ```bash
   docker-compose up -d
   ```

5. Access the application at http://localhost:8080

6. Default admin credentials:
   - Email: admin@example.com
   - Password: changeme123
   - **Important**: Change these immediately after first login!

### Manual Installation

1. Install MySQL and create a database:
   ```sql
   CREATE DATABASE devops_assessment;
   CREATE USER 'devops'@'localhost' IDENTIFIED BY 'your-password';
   GRANT ALL PRIVILEGES ON devops_assessment.* TO 'devops'@'localhost';
   FLUSH PRIVILEGES;
   ```

2. Clone and configure:
   ```bash
   git clone https://github.com/dsgthb/devops-assessment.git
   cd devops-assessment
   cp .env.example .env
   # Edit .env with your database credentials
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Run the application:
   ```bash
   go run cmd/server/main.go
   ```

## Configuration

Key configuration options in `.env`:

- `SERVER_PORT`: Port for the web server (default: 8080)
- `DB_*`: Database connection settings
- `SESSION_SECRET`: Secret key for session encryption (must be at least 32 chars)
- `CSRF_SECRET`: Secret key for CSRF protection
- `QUESTIONS_FILE`: Path to survey questions JSON
- `ADVICE_FILE`: Path to improvement advice JSON

## Usage

### For Users

1. **Login**: Access the application and login with your credentials
2. **Select Team**: Choose the team you want to assess
3. **Complete Survey**: Answer questions across all 7 sections
4. **View Results**: See your maturity scores and improvement areas
5. **Access Resources**: Browse curated learning resources
6. **Export Data**: Download results as CSV for further analysis

### For Administrators

1. **Manage Users**: Create, update, and deactivate user accounts
2. **Manage Teams**: Create teams and assign users
3. **Manage Groups**: Organize teams into groups
4. **View Audit Logs**: Monitor system usage and changes
5. **Assign Roles**: Control access with Admin, Editor, or Viewer roles

## API Documentation

The application provides RESTful APIs:

### Authentication
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/logout` - User logout
- `GET /api/v1/auth/me` - Get current user

### Assessments
- `POST /api/v1/assessments/start` - Start new assessment
- `GET /api/v1/assessments/:id` - Get assessment details
- `POST /api/v1/assessments/:id/sections/:section` - Save section responses
- `POST /api/v1/assessments/:id/complete` - Complete assessment
- `GET /api/v1/assessments/:id/export/csv` - Export to CSV

### Users (Admin only)
- `GET /api/v1/users` - List users
- `POST /api/v1/users` - Create user
- `PUT /api/v1/users/:id` - Update user
- `DELETE /api/v1/users/:id` - Delete user

### Teams
- `GET /api/v1/teams` - List teams
- `POST /api/v1/teams` - Create team
- `PUT /api/v1/teams/:id` - Update team
- `GET /api/v1/teams/:id/members` - Get team members

## Security

- **Authentication**: Session-based with secure tokens
- **Password Storage**: Bcrypt hashing
- **CSRF Protection**: Token-based protection for forms
- **Input Validation**: Server-side validation for all inputs
- **SQL Injection**: Prevented using prepared statements
- **Access Control**: Role-based permissions on all endpoints
- **Audit Trail**: All critical actions are logged

## Development

### Running Tests
```bash
go test ./...
```

### Building for Production
```bash
go build -o devops-assessment ./cmd/server
```

### Database Migrations
Migrations run automatically on startup. To add new migrations, update `internal/database/migrations.go`.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Original PHP version by Atos/Worldline teams
- DevOps community for question contributions
- All contributors who have helped improve this tool

## Support

For issues, questions, or contributions, please use the GitHub issue tracker.