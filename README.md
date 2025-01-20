# Emprius App Backend

Backend service for the Emprius tool sharing platform. This service provides a RESTful API that enables communities to share tools among their members in a managed and organized way.

## Overview

Emprius is a platform that facilitates tool sharing within communities. It allows users to:
- List their tools for others to borrow
- Search for available tools in their area
- Manage tool bookings and returns
- Rate borrowing experiences
- Organize tools by categories and transport options

## Key Features

### User Management
- Community-based user organization
- User profiles with location information
- Avatar image support
- JWT-based authentication
- Invitation-based registration system

### Tool Management
- List tools with detailed information:
  - Title and description
  - Cost and availability options (free/paid)
  - Physical properties (height, weight)
  - Location
  - Transport options
  - Multiple images
- Categorize tools by type
- Search tools by:
  - Location/distance
  - Categories
  - Cost range
  - Transport options
  - Availability

### Booking System
- Request tool bookings with specific dates
- Multiple pending requests support
- Booking workflow:
  - Request → Accept/Deny → Return → Rate
- Conflict prevention for overlapping dates
- Rating system for borrowing experiences

### Image Management
- Upload and store tool images
- Avatar image support for user profiles
- Hash-based image retrieval

## API Documentation

The complete API documentation is available in OpenAPI (Swagger) format:
- Online documentation: [https://emprius.github.io/emprius-app-backend](https://emprius.github.io/emprius-app-backend)
- Local file: [docs/swagger.yaml](docs/swagger.yaml)

## Prerequisites

- Go 1.x
- MongoDB
- Docker (optional)

## Development Setup

1. Clone the repository
2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
- `REGISTER_TOKEN`: Token required for user registration
- `JWT_SECRET`: Secret key for JWT token generation

4. Run the server:
```bash
go run main.go
```

Or using Docker:
```bash
docker-compose up -d
```

## Testing

Run the test suite:
```bash
go test -v ./...
```

Run linting:
```bash
golangci-lint run
```

## License

This project is licensed under the terms of the LICENSE file included in the repository.
