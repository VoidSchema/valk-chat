# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# Copy dependency files first for layer caching
COPY package.json package-lock.json ./
RUN npm ci

# Copy source code
COPY . .

# Accept VITE environment variables during build
ARG VITE_API_URL
ARG VITE_WS_URL
ENV VITE_API_URL=$VITE_API_URL
ENV VITE_WS_URL=$VITE_WS_URL

# Build production bundle
RUN npm run build

# Runtime stage
FROM nginx:alpine

# Remove default nginx config
RUN rm /etc/nginx/conf.d/default.conf

# Copy custom nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy built assets from builder
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
