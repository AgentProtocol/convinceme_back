FROM alpine:3.20 as builder
RUN apk --update add go musl-dev

WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
RUN go mod tidy
COPY . .
ENV GIN_MODE=release
RUN go build -o bin/convinceme -ldflags "-linkmode external -extldflags '-static' -s -w" cmd/main.go

FROM alpine:3.20
COPY --from=builder /app/bin/convinceme /
RUN apk add --no-cache sqlite
COPY sql/ .
COPY . .
# RUN sqlite3 data/arguments.db < sql/schema.sql

ENTRYPOINT ["/convinceme"]
EXPOSE 8080
