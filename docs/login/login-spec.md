

# Technical Specification: JWT Authentication System

## 1. Project Overview
Add new features of user registration and login using email/password. The system must issue a JSON Web Token (JWT) upon successful login. This token acts as a passkey to access a protected faq, uv-advisor and summarizer page.

## 2. Tech Stack Recommendations
* **Database:**  PostgreSQL.
* **Security:** `bcrypt` for password hashing, `jsonwebtoken` for signing.

---

## 3. Database Schema
**Table Name:** `users`

| Column Name | Type | Constraints | Description |
| :--- | :--- | :--- | :--- |
| `id` | Integer/UUID | Primary Key | Unique User ID |
| `email` | String | Unique, Not Null | User email address |
| `nickname` | String | Not Null, <=10 letters | Display name shown in the UI |
| `password` | String | Not Null | **Hashed** password (never plaintext) |
| `created_at` | Timestamp | Default Now | Registration time |

---

## 4. Backend API Specification

### A. Registration Endpoint
* **Route:** `POST /api/auth/register`
* **Input:** JSON body `{ "email": "user@example.com", "password": "password123", "nickname": "Sunshine" }`
* **Logic:**
    1.  Check if `email` already exists. If yes, return 409 Conflict.
    2.  Validate `nickname` (letters only, max 10 chars). If invalid, return 400.
    2.  Hash the password using a salt (e.g., `bcrypt.hash`).
    3.  Insert user into the database.
* **Response:** `201 Created` `{ "message": "User registered successfully", "user": { "email": "...", "nickname": "..." } }`

### B. Login Endpoint
* **Route:** `POST /api/auth/login`
* **Input:** JSON body `{ "email": "user@example.com", "password": "password123" }`
* **Logic:**
    1.  Find user by `email`. If not found, return 401 Unauthorized.
    2.  Compare input password with stored hash (e.g., `bcrypt.compare`). If invalid, return 401.
    3.  **Generate JWT:**
        * **Payload:** `{ "userId": <id>, "email": <email> }`
        * **Secret:** Load from environment variable `JWT_SECRET`.
        * **Algorithm:** HS256.
        * **Expiration:** 1 hour.
* **Response:** `200 OK` `{ "token": "eyJhbGci...", "refreshToken": "..." }`

### C. Refresh Endpoint
* **Route:** `POST /api/auth/refresh`
* **Input:** JSON body `{ "refreshToken": "<refresh_token>" }`
* **Logic:**
    1. Validate the refresh token signature/expiration (longer TTL, e.g. 24h).
    2. Issue a brand new access token + refresh token pair.
* **Response:** `200 OK` `{ "token": "new-access", "refreshToken": "new-refresh" }`

### D. Protected Resource Endpoint
look for faq, summarizer api end points
* **Middleware Requirement:**
    1.  Intercept request.
    2.  Check for `Authorization` header with format `Bearer <token>`.
    3.  Verify token signature. If invalid/expired, return 403 Forbidden.
* **Response:** `200 OK` `{ "message": "Welcome to the private dashboard", "user": { "email": <email>, "nickname": <nickname> } }`

---

## 5. Frontend Specification

### View 1: Authentication Page (`/login`)
* **UI Elements:**
    * A toggle or two forms: **Register** and **Login**.
    * Inputs: Email, Password, Nickname (letters only, max 10 chars).
    * Button: "Submit".
* **Behavior:**
    * **On Register:** Send data to `/register`. Alert success.
    * **On Login:** Send data to `/login`.
    * **Success:** Capture both `token` and `refreshToken` (and the returned nickname) from the response and store them in **localStorage** (`authToken`, `authRefreshToken`, `authNickname`, `authEmail`). Redirect user to Dashboard.
    * **Silent Refresh:** Before an API call fails with 401/403, exchange the stored refresh token via `/api/auth/refresh` and retry automatically.
    * **Error:** Display "Invalid credentials" message.

### View 2: Protected Resource
* **On Load:**
    1.  Check if `authToken` exists in localStorage. If not, redirect to `/login`.
    2.  Make a `GET` request to `/api/summarizer`.
    3.  **Critical:** Include the header `Authorization: Bearer <stored_token>`.
* **UI Elements:**
    * Display the secret message from the backend.
    * Show the logged-in nickname (fallback to email) in the header instead of the raw email.
    * **Logout Button:** When clicked, remove stored auth tokens and redirect to `/login`.

---

## 6. Security Constraints (Instructions for the AI)
1.  **Environment Variables:** Do not hardcode the JWT secret key. Use a `.env` variable from github action.
2.  **Password Safety:** strictly enforce hashing. Never store passwords in plain text.
3.  **CORS:** Ensure the backend allows requests from the frontend origin.
4.  **Refresh Strategy:** Refresh tokens must be stored/rotated securely on the frontend and short-lived access tokens should gate every protected call.

Act as a senior full-stack developer. I need you to implement a secure authentication system based on the following specification.
