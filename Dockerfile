FROM golang as builder

# ENV GOPATH /logx
WORKDIR /logx
COPY . .
RUN sh build.sh

FROM alpine:latest
WORKDIR /logx

COPY --from=builder /logx/bin ./bin
