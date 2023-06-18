# emprius-app-backend

emprius.cat APP backend for sharing resources among communities (WORK IN PROGRESS)



# API Documentation

## Endpoints

### User

#### `GET /api/user/:id`

Returns a JSON object of a user with the provided ID.

```bash
curl -X GET http://localhost:8000/api/user/<id>
```

#### `POST /api/user`

Creates a new user. The body of the request should include username, email, and password.

```bash
curl -X POST -H "Content-Type: application/json" -d '{"username": "<username>", "email": "<email>", "password": "<password>"}' http://localhost:8000/api/user
```

#### PUT /api/user/:id

Updates the details of a user with the provided ID. The body of the request should include any fields to be updated: username, email, password.

```bash
curl -X PUT -H "Content-Type: application/json" -d '{"username": "<new_username>", "email": "<new_email>", "password": "<new_password>"}' http://localhost:8000/api/user/<id>
```

#### `DELETE /api/user/:id`

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

#### `GET /api/tool/:id`

Returns a JSON object of a tool with the provided ID.

```bash
curl -X GET http://localhost:8000/api/tool/<id>
```

    