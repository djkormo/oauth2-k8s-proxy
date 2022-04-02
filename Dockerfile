# All builds should be done using the platform native to the build node to allow
#  cache sharing of the go mod download step.

FROM  golang:1.18-buster AS builder


WORKDIR /app

# Copy sources
COPY *.go ./

# Fetch dependencies
COPY go.mod go.sum ./
# Download packages
RUN go mod download

RUN go build  -o /oauth2-k8s-proxy

# Copy binary to alpine

#FROM alpine:3.15
FROM gcr.io/distroless/base-debian10
COPY nsswitch.conf /etc/nsswitch.conf

COPY --from=builder /oauth2-k8s-proxy /oauth2-k8s-proxy

EXPOSE 8080
USER 2000:2000
ENTRYPOINT ["/oauth2-k8s-proxy"]



