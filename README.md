# emprius-app-backend

emprius.cat APP backend for sharing resources among communities (WORK IN PROGRESS)

## API Endpoints

### 1. Register

**Endpoint:** `/register`

**Method:** `POST`

**Input Parameters:**

- `email`: The email address of the user.
- `name`: The name of the user.
- `password`: The password for the user.
- `invitationToken`: The invitation token for the user.

**Output Model:**

- `header.success`: A boolean indicating whether the operation was successful.
- `data.token`: The token for the user.
- `data.expirity`: The expiration date of the token.

### 2. Login

**Endpoint:** `/login`

**Method:** `POST`

**Input Parameters:**

- `email`: The email of the user.
- `password`: The password of the user.

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.token`: The token for the user.
- `data.expirity`: The expiry date of the token.

### 3. Profile

**Endpoint:** `/profile`

**Method:** `GET`

**Input Parameters:**

- `Authorization`: The bearer token of the user.

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.email`: The email of the user.
- `data.name`: The name of the user.
- `data.community`: The community of the user.
- `data.tokens`: The tokens of the user.
- `data.active`: A boolean indicating if the user is active.
- `data.rating`: The rating of the user.
- `data.avatarHash`: The avatar hash of the user.
- `data.location.latitude`: The latitude of the user's location.
- `data.location.longitude`: The longitude of the user's location.

### 4. Info

**Endpoint:** `/info`

**Method:** `GET`

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.users`: The number of users.
- `data.tools`: The number of tools.
- `data.categories`: The list of available tool categories
- `data.transports`: The list of available transport types.

### 5. Users

**Endpoint:** `/users`

**Method:** `GET`

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.users`: An array of user objects. List all users on the database.

### 6. Images

**Endpoint:** `/images/:hash`

**Method:** `GET`

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.images`: An array of image objects.

### 7. Add image

**Endpoint:** `/images`

**Method:** `POST`

**Input Parameters:**
- `data`: the base64 content of the image.
- `name`: the name for the image.

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.hash`: The Hash identifier for the image.



### 8. Add a new tool

**Endpoint:** `/tools`

**Method:** `POST`

**Input Parameters:**

- `title`: The title of the tool.
- `description`: The description of the tool.
- `mayBeFree`: A boolean indicating if the tool may be free.
- `askWithFee`: A boolean indicating if the tool can be asked with a fee.
- `cost`: The cost of the tool.
- `category`: The category of the tool.
- `estimatedValue`: The estimated value of the tool.
- `height`: The height of the tool.
- `weight`: The weight of the tool.
- `location`: The location of the tool, including `latitude` and `longitude`.

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.id`: The ID of the newly added tool.

### 9. Get tools owned by the user

**Endpoint:** `/tools`

**Method:** `GET`

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.tools`: An array of tool objects owned by the user.

### 10. Modify a tool

**Endpoint:** `/tools/{tool_id}`

**Method:** `PUT`

**Input Parameters:**

- `description`: The new description of the tool.
- `cost`: The new cost of the tool.
- `category`: The new category of the tool.

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.

### 11. Search a tool

**Endpoint:** `/tools/search`

**Method:** `GET`

**Input Parameters:**

- `categories`: An array of categories to search for.
- `maxCost`: The maximum cost of the tools to search for.
- `distance`: The maximum distance to search for tools.

**Output Model:**

- `header.success`: A boolean indicating if the operation was successful.
- `data.tools`: An array of tool objects that match the search criteria. If no body, all objects are returned.