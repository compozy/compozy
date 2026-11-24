FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY sdk/go/go.mod ./sdk/go/go.mod
RUN go mod download github.com/jackc/pgx/v5 github.com/pressly/goose/v3

COPY infra/cloud-edge/build-revision.txt ./cloud-revision.txt
COPY cmd/compozy-cloud ./cmd/compozy-cloud
COPY internal/cloud ./internal/cloud
COPY internal/remoteproto ./internal/remoteproto
COPY packages/brand/cloud_assets.go ./packages/brand/cloud_assets.go
COPY packages/brand/public/brand/visuals/swarm-1280.jpg packages/brand/public/brand/visuals/swarm-portrait-1280.jpg ./packages/brand/public/brand/visuals/
COPY packages/brand/public/brand/svg/compozy-wordmark-white.svg ./packages/brand/public/brand/svg/
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w -X main.cloudRevision=$(cat cloud-revision.txt)" -o /out/compozy-cloud ./cmd/compozy-cloud

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/compozy-cloud /usr/local/bin/compozy-cloud
COPY LICENSE /licenses/MIT
COPY cmd/compozy-cloud/LICENSE /licenses/FSL-1.1-MIT
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/compozy-cloud"]
CMD ["serve"]
