FROM golang:alpine  AS  builder

ENV IP_ADDRESS=127.0.0.1 \
    GOPROXY='https://goproxy.cn,direct' \
    GOOS=linux

WORKDIR /build

COPY . .

RUN go build .

FROM alpine

COPY --from=builder /build/SpringCache /

EXPOSE 9999
EXPOSE 8888

WORKDIR /

CMD ["/bin/sh","-c","/SpringCache --name svc --peer svc --etcd 127.0.0.1:2379"]