# Stage 1: Build the Go application
FROM golang:1.18.1-alpine AS build_base
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN go build -o ./bin/apiserver .

# Stage 2: Create a lightweight production container
FROM alpine:3.16.0
RUN apk update && apk add --no-cache tzdata bash
ENV TZ=Asia/Kolkata
COPY --from=build_base /app/bin/apiserver /apiserver
LABEL org.opencontainers.image.source="https://github.com/physizAg/apiserver"
EXPOSE 3000
CMD [ "/apiserver" ]
