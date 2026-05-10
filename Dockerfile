FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/derek-relay ./cmd/derek-relay

FROM alpine:3.20

RUN adduser -D -H relay
USER relay
EXPOSE 18080
COPY --from=build /out/derek-relay /usr/local/bin/derek-relay
ENTRYPOINT ["derek-relay"]
