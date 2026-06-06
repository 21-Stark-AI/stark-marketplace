# stark-marketplace static origin — built and deployed by .github/workflows/web-deploy.yml.
#
# The web/dist directory (Vite SPA + staged index.json/bundles) is built by the
# workflow BEFORE `docker build`, then copied in here as one content-hashed unit
# (spec §10). The server is a tiny stdlib-only Go binary (server/), so the final
# image is distroless/static + the binary + /public. No web build happens in the
# image — keep the lint/typecheck/test gates in CI authoritative.

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY server/go.mod ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
# Built by the workflow before `docker build`; served from WEBROOT.
COPY web/dist /app/public
ENV WEBROOT=/app/public PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
