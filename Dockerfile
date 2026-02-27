FROM golang:1.26-alpine AS builder
ARG MAKE_TARGET=release
RUN apk add --no-cache make git upx
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 make ${MAKE_TARGET}

FROM alpine:3.22
RUN apk add --no-cache wget && adduser -D gimme

WORKDIR /app
COPY --chown=gimme:gimme --from=builder /build/dist/gimme     /bin/gimme
COPY --chown=gimme:gimme --from=builder /build/dist/templates /app/templates
COPY --chown=gimme:gimme --from=builder /build/dist/docs      /app/docs

ENV GIN_MODE=release

EXPOSE 8080

# HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
#   CMD ["wget", "-qO-",  "http://localhost:8080/"] || exit 1

USER gimme

ENTRYPOINT ["/bin/gimme"]
