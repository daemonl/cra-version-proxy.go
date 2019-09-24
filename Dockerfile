FROM golang:1.13
# AS builder
COPY *.go ./
RUN go test *.go
RUN go build -o /proxy-server ./main.go

#FROM alpine:latest
#RUN apk --no-cache add ca-certificates

#COPY --from=builder /proxy-server /

ENV CRA_PROXY_CACHE_DIR=/data/cache
RUN mkdir -p /data/cache

ENV CRA_PROXY_BIND=:80 
EXPOSE 80

#ENV CRA_PROXY_SOURCE
#ENV CRA_PROXY_DEFAULT_VERSION

CMD ["/proxy-server"]
