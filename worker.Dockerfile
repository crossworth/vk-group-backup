FROM golang:1.13-alpine as build

ENV CGO_ENABLED=0

RUN apk add --no-cache git

COPY go.mod go.sum /app/

RUN cd /app && go mod download

COPY . /app

RUN cd /app && go build -o worker cmd/worker/main.go

FROM alpine:latest

COPY --from=build /app/worker /home/worker

RUN chmod +x /home/worker

CMD ["/home/worker"]
