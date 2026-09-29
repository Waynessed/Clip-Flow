FROM node:24.21.0-bookworm-slim
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build
ENV API_URL=http://api:8080
CMD ["npm","run","dev"]
