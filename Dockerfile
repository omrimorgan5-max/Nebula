FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/nebula .

FROM alpine:3.21
RUN addgroup -S -g 65532 nebula && adduser -S -D -H -u 65532 -G nebula nebula \
    && mkdir -p /data && chown nebula:nebula /data
COPY --from=build /out/nebula /usr/local/bin/nebula
ENV ADDR=:8080 DATA_DIR=/data COOKIE_SECURE=auto
EXPOSE 8080
USER nebula
ENTRYPOINT ["/usr/local/bin/nebula"]