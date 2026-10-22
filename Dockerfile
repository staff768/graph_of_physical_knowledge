FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/seed ./cmd/seed

FROM alpine:3.22
WORKDIR /app
COPY --from=go /out/api /out/seed ./
COPY --from=web /web/dist ./web
COPY content ./content
EXPOSE 8080
CMD ["/app/api"]
