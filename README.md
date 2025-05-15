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

## API Examples

Here are some basic curl examples to get started with the API. Replace `localhost:3333` with your server's address.

### Authentication

1. Register a new user:
```bash
curl -X POST http://localhost:3333/register -H 'Content-Type: application/json' -d '{
  "email": "user@example.com",
  "name": "Test User",
  "password": "userpass123",
  "invitationToken": "comunals"
}'
```

2. Login and get JWT token:
```bash
curl -X POST http://localhost:3333/login -H 'Content-Type: application/json' -d '{
  "email": "user@example.com",
  "password": "userpass123"
}'
```

Save the JWT token for subsequent requests:
```bash
export TOKEN="your_jwt_token_here"
```

### User Profile

1. Get user profile:
```bash
curl http://localhost:3333/profile -H "Authorization: BEARER $TOKEN"
```

2. Update profile:
```bash
curl -X POST http://localhost:3333/profile \
  -H "Authorization: BEARER $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "location": {
      "latitude": 42202259,
      "longitude": 1815044
    },
    "community": "Example Community"
  }'
```

### Tools

1. Add a new tool:
```bash
curl -X POST http://localhost:3333/tools \
  -H "Authorization: BEARER $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Hammer",
    "description": "A useful tool",
    "mayBeFree": true,
    "askWithFee": false,
    "cost": 10,
    "transportOptions": [1, 2],
    "category": 1,
    "location": {
      "latitude": 42202259,
      "longitude": 1815044
    },
    "toolValuation": 20,
    "height": 30,
    "weight": 40
  }'
```

2. Search for tools:
```bash
curl "http://localhost:3333/tools/search?categories=1,2&maxCost=100&distance=20000&mayBeFree=true" \
  -H "Authorization: BEARER $TOKEN"
```

### Bookings

1. Create a booking request:
```bash
curl -X POST http://localhost:3333/bookings \
  -H "Authorization: BEARER $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "toolId": "123456",
    "startDate": '$(date -d "+1 day" +%s)',
    "endDate": '$(date -d "+2 days" +%s)',
    "contact": "user@example.com",
    "comments": "I need this tool for a project"
  }'
```

2. Accept a booking request (tool owner only):
```bash
curl -X POST http://localhost:3333/bookings/petitions/{bookingId}/accept \
  -H "Authorization: BEARER $TOKEN"
```

3. Mark a booking as returned:
```bash
curl -X POST http://localhost:3333/bookings/{bookingId}/return \
  -H "Authorization: BEARER $TOKEN"
```

4. Rate a booking:
```bash
curl -X POST http://localhost:3333/bookings/rates \
  -H "Authorization: BEARER $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "bookingId": "booking_id_here",
    "rating": 5
  }'
```

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
