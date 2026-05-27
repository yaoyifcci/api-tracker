FROM node:20-alpine AS builder
WORKDIR /app
ARG HTTPS_PROXY=""
ARG HTTP_PROXY=""
ARG http_proxy=""
ARG https_proxy=""
ENV HTTPS_PROXY=""
ENV HTTP_PROXY=""
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
