
FROM golang:1.10.4-alpine3.8

WORKDIR /go/src/github.com/LeKovr/macobot
RUN apk --update add curl git
ADD . .
RUN curl https://glide.sh/get | sh
RUN glide install
RUN go install github.com/LeKovr/macobot

FROM alpine:3.8

MAINTAINER Aleksey Kovrizhkin <lekovr@gmail.com>

ENV DOCKERFILE_VERSION  180911

RUN apk --update add curl make coreutils diffutils gawk git openssl postgresql-client bash

WORKDIR /opt/macobot

COPY --from=0 /go/bin/macobot .

CMD ["/opt/macobot/macobot"]
