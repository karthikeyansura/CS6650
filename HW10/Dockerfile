FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod .
COPY ["cmd", "./cmd"]
COPY internal ./internal
RUN go build -o /bin/kvserver ./cmd/server

FROM alpine:3.20
RUN adduser -D app
USER app
COPY --from=build /bin/kvserver /bin/kvserver
EXPOSE 8080
ENTRYPOINT ["/bin/kvserver"]
