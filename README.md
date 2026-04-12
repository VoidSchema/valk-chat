# Valk Chat

Valk Chat is a real-time chat application featuring user authentication, rate limiting, and 24-hour expiration of chat messages. It is containerized using Docker, with a Golang backend and a modern React frontend.

## Features
- Real-time chat via WebSockets
- User Registration & Login with Password & Cookies
- Rate limiting to protect the application
- Chat messages expire after 24 hours
- Fully Dockerized environment

## Prerequisites
- Docker & Docker Compose

## Running the Application
1. Clone the repository.
2. Ensure you have your `.env` configured inside the `backend` directory (do not commit this file).
3. Run the following command at the root of the project to start all services:
   ```bash
   docker-compose up --build
   ```
4. Access the application at `http://localhost:8080`.
