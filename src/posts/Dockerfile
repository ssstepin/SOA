FROM golang:1.23-alpine

WORKDIR /social-network/posts
COPY go.* .

RUN go mod download -x

COPY * .

RUN go build

ENTRYPOINT ["./posts"]
