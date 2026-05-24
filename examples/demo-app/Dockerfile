FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod .
# go mod tidy downloads deps and generates go.sum inside the build layer
COPY . .
RUN go mod tidy && go build -o /demo-app .

FROM alpine:latest
COPY --from=builder /demo-app /demo-app
EXPOSE 8080
CMD ["/demo-app"]