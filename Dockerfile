FROM golang:1-alpine as builder
RUN apk update && apk add gcc make g++ git
WORKDIR /build
ADD . .
RUN make build

FROM alpine
COPY --from=builder /build/dist/gimme /bin/gimme
COPY --from=builder /build/dist/templates /templates
COPY --from=builder /build/dist/docs /docs
RUN chmod +x /bin/gimme

ENV GIN_MODE=release

ENTRYPOINT ["/bin/gimme"]
