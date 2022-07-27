FROM golang:latest as builder
WORKDIR /usr/app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build .

FROM gcr.io/distroless/static
COPY --from=builder /usr/app/redditalert ./
COPY config.json agent.yaml ./
CMD ["./redditalert", "--logtostderr"]
