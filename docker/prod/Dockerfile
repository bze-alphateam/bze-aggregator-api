# Production image. One image serves both long-running roles:
#   docker run <img>                  -> HTTP API (no args, port 8888)
#   docker run <img> sync listener    -> websocket listener/consumer
# plus one-shot commands (cleanup, sync ...) via `docker compose run`.
#
# The app reads ALL configuration from a `.env` file in its working
# directory (godotenv) — not from process environment variables. Mount the
# runtime .env at /app/.env.

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /bze-agg .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /bze-agg /bze-agg
EXPOSE 8888
ENTRYPOINT ["/bze-agg"]
