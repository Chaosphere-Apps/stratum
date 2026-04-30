# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/backend ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /out/backend /backend

EXPOSE 8081

ENV BACKEND_ADDR=:8081

ENTRYPOINT ["/backend"]

