FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY sdk/go/go.mod ./sdk/go/go.mod
RUN go mod download github.com/jackc/pgx/v5 github.com/pressly/goose/v3

COPY cmd/compozy-cloud ./cmd/compozy-cloud
COPY internal/cloud ./internal/cloud
COPY internal/remoteproto ./internal/remoteproto
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/compozy-cloud ./cmd/compozy-cloud

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/compozy-cloud /usr/local/bin/compozy-cloud
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/compozy-cloud"]
CMD ["serve"]
