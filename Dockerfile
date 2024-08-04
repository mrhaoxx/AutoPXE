FROM docker.io/library/golang:1.22-alpine as builder

COPY . /go/src/github.com/mrhaoxx/AutoPXE

RUN cd /go/src/github.com/mrhaoxx/AutoPXE && go env -w GOPROXY=https://goproxy.cn,direct && go build -o /AutoPXE

FROM scratch

COPY --from=builder /AutoPXE /AutoPXE

ENTRYPOINT ["/AutoPXE"]