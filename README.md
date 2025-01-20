# Emprius App Backend

A RESTful API backend service for the Emprius tool sharing platform. This service provides endpoints for user management, tool sharing, and image handling.

[Previous content remains the same until the tool response example...]

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
        "userId": "507f1f77bcf86cd799439011",
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

[Previous content remains the same until the booking response example...]

Response:
```json
{
  "header": {
    "success": true
  },
  "data": {
    "id": "6773e44f06307bedd602fbd2",
    "toolId": "123456",
    "fromUserId": "507f1f77bcf86cd799439011",
    "toUserId": "507f1f77bcf86cd799439012",
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

[Previous content remains the same until the ratings response example...]

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
        "fromUserId": "507f1f77bcf86cd799439011",
        "toUserId": "507f1f77bcf86cd799439012",
        "isPending": true,
        "ratingType": "tool"
      }
    ]
  }
}
```

[Rest of the content remains exactly the same...]
