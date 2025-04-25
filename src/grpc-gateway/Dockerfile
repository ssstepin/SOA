FROM golang:1.23-alpine

WORKDIR /social-network/grpc-gateway
COPY go.* .

RUN go mod download -x

COPY * .

RUN go build

ENTRYPOINT ["./grpc-gateway"]
