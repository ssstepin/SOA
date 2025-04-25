FROM golang:1.20-alpine

WORKDIR /social-network/gateway
COPY go.* .

RUN go mod download -x

COPY * .

RUN go build .

ENTRYPOINT ["./gateway"]
