# Build stage
FROM node:20 AS builder

# Accept build arguments
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install

COPY . .

# Set version as environment variable for build process
ENV VITE_APP_VERSION=$VERSION
ENV VITE_GIT_COMMIT=$GIT_COMMIT
ENV VITE_BUILD_DATE=$BUILD_DATE

RUN npm run build

# Production stage
FROM nginx:alpine

# Re-declare ARGs after FROM to use them in this stage
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

# Set version as environment variable for runtime access
ENV APP_VERSION=$VERSION
ENV GIT_COMMIT=$GIT_COMMIT
ENV BUILD_DATE=$BUILD_DATE

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]

