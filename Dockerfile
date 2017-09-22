FROM golang:alpine

MAINTAINER Kristoph Junge <kristoph.junge@gmail.com>

RUN apk update && apk upgrade && \
    apk add --no-cache git

WORKDIR /go

COPY . .

RUN go get github.com/rogpeppe/rog-go/reverse && \
    go build -v -o bin/app src/app.go

VOLUME /var/log/ethminer.log

CMD ["./bin/app"]