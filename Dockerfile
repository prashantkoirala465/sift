FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/sift ./cmd/sift

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 sift
COPY --from=build /out/sift /usr/local/bin/sift

USER sift
EXPOSE 8080
ENTRYPOINT ["sift"]
