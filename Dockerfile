# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/jodbanchee ./cmd/jodbanchee \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/jodbanchee /jodbanchee
# Owned by the nonroot user so a fresh Docker volume mounted here is writable.
COPY --from=build --chown=65532:65532 /out/data /data
ENV DATA_FILE=/data/ledger.json PORT=8080
VOLUME ["/data"]
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/jodbanchee"]
