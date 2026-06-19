# Build
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/eywa-starter .

# Run
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/eywa-starter /eywa-starter
ENTRYPOINT ["/eywa-starter"]
