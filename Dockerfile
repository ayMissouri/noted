FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /noted ./cmd/noted

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 1000 noted \
    && mkdir -p /data \
    && chown noted /data
COPY --from=build /noted /usr/local/bin/noted
USER noted
ENV NOTED_DATA_DIR=/data
ENV NOTED_LISTEN_ADDR=:6683
EXPOSE 6683
ENTRYPOINT ["noted"]
