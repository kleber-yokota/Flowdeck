# Go 1.27 (último estável; DBOS v1.7.0 exige go >= 1.25)
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o flowdeck .

FROM gcr.io/distroless/static
COPY --from=build /src/flowdeck /flowdeck
EXPOSE 8080
ENTRYPOINT ["/flowdeck"]
