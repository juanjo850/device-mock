FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/device-mock ./cmd/device-mock

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/device-mock /device-mock
COPY configs /configs
EXPOSE 8080 9000 9001
ENTRYPOINT ["/device-mock", "-config", "/configs/devices.json"]
