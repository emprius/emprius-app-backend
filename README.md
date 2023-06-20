# emprius-app-backend

emprius.cat APP backend for sharing resources among communities (WORK IN PROGRESS)



# API Documentation

## Endpoints

### Info

#### `GET /info`

`curl localhost:3333/info`

```
{
  "header": {
    "success": true
  },
  "data": {
    "users": 0,
    "tools": 0,
    "categories": [
      {
        "id": 1,
        "name": "other"
      },
      {
        "id": 2,
        "name": "transport"
      },
      {
        "id": 3,
        "name": "construction"
      },
      {
        "id": 4,
        "name": "agriculture"
      },
      {
        "id": 5,
        "name": "communication"
      }
    ],
    "transports": [
      {
        "id": 1,
        "name": "Car"
      },
      {
        "id": 2,
        "name": "Van"
      },
      {
        "id": 3,
        "name": "Truck"
      }
    ]
  }
}
```

### User

#### `GET /user/:id`

Returns a JSON object of a user with the provided ID.

```bash
curl http://localhost:3333/user/<id>
```

#### `POST /user`

Creates a new user. The body of the request should include username, email, and password.

```bash
curl -X POST -H "Content-Type: application/json" -d '{"username": "<username>", "email": "<email>", "password": "<password>"}' http://localhost:3333/user
```

#### PUT /api/user/:id

Updates the details of a user with the provided ID. The body of the request should include any fields to be updated: username, email, password.

```bash
curl -X PUT -H "Content-Type: application/json" -d '{"username": "<new_username>", "email": "<new_email>", "password": "<new_password>"}' http://localhost:8000/api/user/<id>
```

#### `DELETE /user/:id`

Deletes a user with the provided ID.

```bash
curl -X DELETE http://localhost:8000/api/user/<id>
```

### Image

#### `GET /api/image/:hash`
Returns a JSON object of an image with the provided hash.

```bash
curl -X GET http://localhost:8000/api/image/<hash>
```

#### `POST /api/image`

Uploads a new image. The body of the request should include hash, name, content and link.

```bash
curl -X POST -H "Content-Type: application/json" -d '{"hash": "<hash>", "name": "<name>", "content": "<content>", "link": "<link>"}' http://localhost:8000/api/image
```

#### `DELETE /api/image/:hash`

Deletes an image with the provided hash.

```bash
curl -X DELETE http://localhost:8000/api/image/<hash>
```

### Transport

#### `GET /api/transport/:id`

Returns a JSON object of a transport method with the provided ID.

```bash
curl -X GET http://localhost:8000/api/transport/<id>
```

#### POST /api/transport

Creates a new transport method. The body of the request should include id and name.

```bash
curl -X POST -H "Content-Type: application/json" -d '{"id": "<id>", "name": "<name>"}' http://localhost:8000/api/transport
```

#### `PUT /api/transport/:id`

Updates the details of a transport method with the provided ID. The body of the request should include id and/or name.

```bash
curl -X PUT -H "Content-Type: application/json" -d '{"id": "<new_id>", "name": "<new_name>"}' http://localhost:8000/api/transport/<id>
```

#### `DELETE /api/transport/:id`

Deletes a transport method with the provided ID.

```bash
curl -X DELETE http://localhost:8000/api/transport/<id>
```

### Tool

+ GET /tools - Get tools owned by the user

curl https://api.emprius.com/tools'

+ GET /tools/:id - Get a tool by id

curl -X GET 'https://api.emprius.com/tools/{id}

+ GET /tools/user/:id - Get tools owned by a specific user

curl -X GET 'https://api.emprius.com/tools/user/{email}

+ GET /tools/search - Filter tools

curl -X GET 'https://api.emprius.com/tools/search' \
-d '{
  "Categories": [1, 2],
  "MayBeFree": true,
  "MaxCost": 100,
  "Distance": 20000
}'

+ POST /tools - Add a new tool

curl -X POST 'https://api.emprius.com/tools' \
-d '{
  "Title": "Hammer",
  "Description": "A useful tool",
  "MayBeFree": true,
  "AskWithFee": false,
  "Cost": 10,
  "Category": 1,
  "EstimatedValue": 20,
  "Height": 30,
  "Weight": 40,
  "Images": ["image1base64Hash", "image2base64Hash"],
  "Location": {
    "Latitude": 50000000,
    "Longitude": 50000000
  }
}'

+ DELETE /tools/:id - Delete a tool

curl -X DELETE 'https://api.emprius.com/tools/{id}'

+ PUT /tools/:id - Edit a tool

curl -X PUT 'https://api.emprius.com/tools/{id}' \
-d '{
  "Title": "New Title",
  "Description": "New Description",
  "MayBeFree": false,
  "AskWithFee": true,
  "Cost": 20,
  "Category": 2,
  "EstimatedValue": 30,
  "Height": 40,
  "Weight": 50,
  "Images": ["new_image1base64hash"],
  "Location": {
    "Latitude": 60000000,
    "Longitude": 60000000
  }
}'


