FROM golang:1.13-alpine as build

ENV CGO_ENABLED=0

RUN apk add --no-cache git

COPY go.mod go.sum /app/

RUN cd /app && go mod download

COPY . /app

RUN cd /app && go build -o queue-all-topics cmd/queue-all-topics/main.go

FROM alpine:latest

COPY --from=build /app/queue-all-topics /home/queue-all-topics

RUN chmod +x /home/queue-all-topics

CMD ["/home/queue-all-topics"]
