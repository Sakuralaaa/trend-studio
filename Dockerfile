FROM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
FROM golang:1.27.1-bookworm AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/trend-studio ./cmd/trend-studio
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -trimpath -o /out/test-provider ./cmd/test-provider
FROM debian:bookworm-slim AS fonts
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl && rm -rf /var/lib/apt/lists/*
COPY fonts/source.lock /tmp/source.lock
RUN mkdir /fonts && url=$(sed -n '1p' /tmp/source.lock) && sha=$(sed -n '2p' /tmp/source.lock) && curl --fail -L "$url" -o /fonts/NotoSansSC.ttf && echo "$sha  /fonts/NotoSansSC.ttf" | sha256sum -c -
COPY fonts/OFL.txt /fonts/OFL.txt
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=backend /out/trend-studio /app/trend-studio
COPY --from=web /src/web/dist /app/web
COPY --from=fonts /fonts /app/fonts
ENV DATA_DIR=/data WEB_DIR=/app/web FONT_PATH=/app/fonts/NotoSansSC.ttf LISTEN_ADDR=:8080
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/trend-studio"]
CMD ["api"]
FROM gcr.io/distroless/static-debian12:nonroot AS fixture
COPY --from=backend /out/test-provider /test-provider
EXPOSE 8091
ENTRYPOINT ["/test-provider"]
