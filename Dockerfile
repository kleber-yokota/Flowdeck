# Go 1.27.3 — mesma versão usada nos testes (DBOS v1.7.0 exige go >= 1.25)
FROM golang:1.27.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o flowdeck .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /src/flowdeck /flowdeck
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/flowdeck"]
