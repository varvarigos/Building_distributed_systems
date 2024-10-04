# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /video-rec-service ./video_rec_service/server

FROM alpine:latest AS deployment-stage

COPY --from=build-stage /video-rec-service /video_rec_service

EXPOSE 8080

CMD [ "/video-rec-service" ]