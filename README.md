# Emprius App Backend

A RESTful API backend service for the Emprius tool sharing platform. This service provides endpoints for user management, tool sharing, and image handling.

## Features

- User authentication with JWT tokens
- User profile management with avatar support
- Tool management (add, edit, delete, search)
- Image upload and retrieval
- Location-based tool search
- Community-based user organization

## Prerequisites

- Go 1.x
- MongoDB
- Docker (optional)

## Installation

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

Or using Docker (for testing):
```bash
docker-compose up -d
```

# API Documentation

The API uses JWT (JSON Web Token) for authentication. Most endpoints require a valid JWT token in the Authorization header:

```
Authorization: BEARER <your-jwt-token>
```

## Public Endpoints

### POST /register
Register a new user.

Curl example:
```bash
curl -X POST http://localhost:3333/register \
  -d '{
    "email": "user@example.com",
    "name": "Username",
    "password": "userpassword",
    "invitationToken": "comunals"
  }'
```

Request:
```json
{
  "email": "user@example.com",
  "invitationToken": "required-token",
  "name": "john smith",
  "community": "Community1",
  "location": {
    "latitude": 42202259,
    "longitude": 1815044
  },
  "password": "userpassword",
  "avatar": "base64-encoded-image"
}
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "token": "jwt-token",
    "expirity": "2024-01-01T00:00:00Z"
  }
}
```

### POST /login
Authenticate user and get JWT token.

Curl example:
```bash
curl -X POST http://localhost:3333/login \
  -d '{
    "email": "user@example.com",
    "password": "userpassword"
  }'
```

Request:
```json
{
  "email": "user@example.com",
  "password": "userpassword"
}
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "token": "jwt-token",
    "expirity": "2024-01-01T00:00:00Z"
  }
}
```

### GET /info
Get general platform information.

Curl example:
```bash
curl http://localhost:3333/info
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "users": 100,
    "tools": 250,
    "categories": [
      {"id": 1, "name": "Category1"},
      {"id": 2, "name": "Category2"}
    ],
    "transports": [
      {"id": 1, "name": "Transport1"},
      {"id": 2, "name": "Transport2"}
    ]
  }
}
```

## Protected Endpoints (Require Authentication)

### GET /profile
Get user profile information.

Curl example:
```bash
curl http://localhost:3333/profile \
  -H "Authorization: BEARER your-jwt-token"
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "email": "user@example.com",
    "name": "Username",
    "community": "Community1",
    "location": {
      "latitude": 42202259,
      "longitude": 1815044
    },
    "active": true,
    "avatarHash": "image-hash"
  }
}
```

### POST /profile
Update user profile.

Curl example:
```bash
curl -X POST http://localhost:3333/profile \
  -H "Authorization: BEARER your-jwt-token" \
  -d '{
    "location": {
      "latitude": 42202259,
      "longitude": 1815044
    },
    "community": "Karabanchel"
  }'
```

Request:
```json
{
  "name": "New Name",
  "community": "New Community",
  "location": {
    "latitude": 42202259,
    "longitude": 1815044
  },
  "avatar": "base64-encoded-image"
}
```

### GET /users
List all users.

Curl example:
```bash
curl http://localhost:3333/users \
  -H "Authorization: BEARER your-jwt-token"
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "users": [
      {
        "email": "user1@example.com",
        "name": "User1",
        "community": "Community1"
      }
    ]
  }
}
```

### POST /tools
Add a new tool.

Curl example:
```bash
curl -X POST http://localhost:3333/tools \
  -H "Authorization: BEARER your-jwt-token" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Hammer",
    "description": "A useful tool",
    "mayBeFree": true,
    "askWithFee": false,
    "cost": 10,
    "images": [],
    "transportOptions": [1, 2],
    "category": 1,
    "location": {
      "latitude": 42202259,
      "longitude": 1815044
    },
    "estimatedValue": 20,
    "height": 30,
    "weight": 40
  }'
```

Request:
```json
{
  "id": 0,
  "title": "Tool Name",
  "description": "Tool Description",
  "mayBeFree": true,
  "askWithFee": false,
  "cost": 10,
  "images": [],
  "transportOptions": [1, 2],
  "category": 1,
  "location": {
    "latitude": 42202259,
    "longitude": 1815044
  },
  "estimatedValue": 20,
  "height": 30,
  "weight": 40
}
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "id": 123456
  }
}
```

### GET /tools
Get tools owned by the authenticated user.

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "tools": [
      {
        "id": 123456,
        "title": "Tool Name",
        "description": "Tool Description",
        "mayBeFree": true,
        "askWithFee": false,
        "cost": 10,
        "images": [],
        "transportOptions": [1, 2],
        "category": 1,
        "location": {
          "latitude": 42202259,
          "longitude": 1815044
        },
        "estimatedValue": 20,
        "height": 30,
        "weight": 40
      }
    ]
  }
}
```

### GET /tools/{id}
Get specific tool details.

### PUT /tools/{id}
Update tool information.

Curl example:
```bash
curl -X PUT http://localhost:3333/tools/tool-id \
  -H "Authorization: BEARER your-jwt-token" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "New Description",
    "cost": 20,
    "category": 2
  }'
```

Request:
```json
{
  "description": "New Description",
  "cost": 20,
  "category": 2
}
```

### DELETE /tools/{id}
Delete a tool.

### GET /tools/search
Search for tools with filters.

Curl example:
```bash
curl -X GET http://localhost:3333/tools/search \
  -H "Authorization: BEARER your-jwt-token" \
  -H "Content-Type: application/json" \
  -d '{
    "categories": [1, 2],
    "maxCost": 100,
    "distance": 20000,
    "mayBeFree": true
  }'
```

Request:
```json
{
  "categories": [1, 2],
  "maxCost": 100,
  "distance": 20000,
  "mayBeFree": true
}
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "tools": [
      {
        "id": 123456,
        "title": "Tool Name",
        "description": "Tool Description",
        "cost": 10,
        "category": 1
      }
    ]
  }
}
```

### POST /bookings
Create a new booking request.

Curl example:
```bash
curl -X POST http://localhost:3333/bookings \
  -H "Authorization: BEARER your-jwt-token" \
  -H "Content-Type: application/json" \
  -d '{
    "toolId": "tool-id",
    "startDate": 1735734735,
    "endDate": 1735821135,
    "contact": "user@example.com",
    "comments": "I need this tool for a project"
  }'
```

Request:
```json
{
  "toolId": "123456",
  "startDate": 1735734735,
  "endDate": 1735821135,
  "contact": "test@example.com",
  "comments": "I need this tool for a weekend project"
}
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "id": "6773e44f06307bedd602fbd2",
    "toolId": "123456",
    "fromUserId": "user123",
    "toUserId": "owner456",
    "startDate": 1735734735,
    "endDate": 1735821135,
    "contact": "test@example.com",
    "comments": "I need this tool for a weekend project",
    "bookingStatus": "pending",
    "createdAt": "2024-01-01T00:00:00Z",
    "updatedAt": "2024-01-01T00:00:00Z"
  }
}
```

### GET /bookings/requests
Get list of booking requests for tools you own.

Curl example:
```bash
curl http://localhost:3333/bookings/requests \
  -H "Authorization: BEARER your-jwt-token"
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "bookings": [
      {
        "id": "6773e44f06307bedd602fbd2",
        "toolId": "123456",
        "fromUserId": "user123",
        "toUserId": "owner456",
        "startDate": 1735734735,
        "endDate": 1735821135,
        "contact": "test@example.com",
        "comments": "I need this tool for a weekend project",
        "bookingStatus": "pending",
        "createdAt": "2024-01-01T00:00:00Z",
        "updatedAt": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

### GET /bookings/petitions
Get list of your booking requests.

Curl example:
```bash
curl http://localhost:3333/bookings/petitions \
  -H "Authorization: BEARER your-jwt-token"
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "bookings": [
      {
        "id": "6773e44f06307bedd602fbd2",
        "toolId": "123456",
        "fromUserId": "user123",
        "toUserId": "owner456",
        "startDate": 1735734735,
        "endDate": 1735821135,
        "contact": "test@example.com",
        "comments": "I need this tool for a weekend project",
        "bookingStatus": "pending",
        "createdAt": "2024-01-01T00:00:00Z",
        "updatedAt": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

### GET /bookings/{bookingId}
Get specific booking details.

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "id": "6773e44f06307bedd602fbd2",
    "toolId": "123456",
    "fromUserId": "user123",
    "toUserId": "owner456",
    "startDate": 1735734735,
    "endDate": 1735821135,
    "contact": "test@example.com",
    "comments": "I need this tool for a weekend project",
    "bookingStatus": "pending",
    "createdAt": "2024-01-01T00:00:00Z",
    "updatedAt": "2024-01-01T00:00:00Z"
  }
}
```

### POST /bookings/{bookingId}/return
Mark a tool as returned (tool owner only).

Curl example:
```bash
curl -X POST http://localhost:3333/bookings/booking-id/return \
  -H "Authorization: BEARER your-jwt-token"
```

Response:
```json
{
  "header": {
    "success": true
  }
}
```

### GET /bookings/rates
Get list of pending ratings.

Curl example:
```bash
curl http://localhost:3333/bookings/rates \
  -H "Authorization: BEARER your-jwt-token"
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "ratings": [
      {
        "id": "6773e44f06307bedd602fbd2",
        "bookingId": "6773e44f06307bedd602fbd2",
        "fromUserId": "user123",
        "toUserId": "owner456",
        "isPending": true,
        "ratingType": "tool"
      }
    ]
  }
}
```

### POST /bookings/rates
Submit a rating for a booking.

Curl example:
```bash
curl -X POST http://localhost:3333/bookings/rates \
  -H "Authorization: BEARER your-jwt-token" \
  -H "Content-Type: application/json" \
  -d '{
    "bookingId": "booking-id",
    "rating": 5
  }'
```

Request:
```json
{
  "bookingId": "6773e44f06307bedd602fbd2",
  "rating": 5
}
```

Response:
```json
{
  "header": {
    "success": true
  }
}
```


### POST /images
Upload an image.

Curl example:
```bash
curl -X POST http://localhost:3333/images \
  -H "Authorization: BEARER your-jwt-token" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "base64-encoded-image"
  }'
```

Request:
```json
{
  "name": "image-name",
  "content": "base64-encoded-image"
}
```

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "hash": "image-hash"
  }
}
```

### GET /images/{hash}
Get an image by its hash.

Curl example:
```bash
curl http://localhost:3333/images/image-hash \
  -H "Authorization: BEARER your-jwt-token"
```

## Error Responses

All endpoints return errors in the following format:

```json
{
  "header": {
    "success": false,
    "message": "Error description",
    "errorCode": 123
  }
}
```

Common error messages:

Authentication & General:
- Invalid register auth token
- Invalid request body data
- Could not insert to database
- Wrong password or email
- Invalid JSON body

Resources:
- Invalid hash
- Image not found
- Tool not found

Bookings:
- Booking dates conflict with existing booking
- Unauthorized booking operation
- Invalid booking dates
- Booking not found
- Only tool owner can mark as returned
- Booking already marked as returned
- Invalid rating value
- Booking already rated

# Development

### Running Tests

```bash
go test ./...
```

### API Testing Script

A test script (`test.sh`) is provided to demonstrate API usage. Run it with:

```bash
./test.sh
```

## License

This project is licensed under the terms of the LICENSE file included in the repository.
