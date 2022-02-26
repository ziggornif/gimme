FROM golang:1-alpine as builder
RUN apk update && apk add gcc make g++
WORKDIR /build
ADD . .
RUN make build

FROM alpine
COPY --from=builder /build/gimme /bin/gimme
RUN chmod +x /bin/gimme
COPY --from=builder /build/gimme-conf/gimme.yml /config/gimme.yml

ENV GIN_MODE=release

ENTRYPOINT ["/bin/gimme"]
