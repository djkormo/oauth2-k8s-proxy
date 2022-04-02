# All builds should be done using the platform native to the build node to allow
#  cache sharing of the go mod download step.
# Go cross compilation is also faster than emulation the go compilation across
#  multiple platforms.
FROM --platform=${BUILDPLATFORM} golang:1.18-buster AS builder

# Copy sources
WORKDIR $GOPATH/src/github.com/djkormo/oauth2-k8s-proxy

# Fetch dependencies
COPY go.mod go.sum ./
RUN go mod download

# Now pull in our code
COPY . .

# Arguments go here so that the previous steps can be cached if no external
#  sources have changed.
ARG VERSION=0.1.0
ARG TARGETPLATFORM=linux/amd64
ARG BUILDPLATFORM=linux/amd64

# Build binary and make sure there is at least an empty key file.
#  This is useful for GCP App Engine custom runtime builds, because
#  you cannot use multiline variables in their app.yaml, so you have to
#  build the key into the container and then tell it where it is
#  by setting OAUTH2_PROXY_JWT_KEY_FILE=/etc/ssl/private/jwt_signing_key.pem
#  in app.yaml instead.
# Set the cross compilation arguments based on the TARGETPLATFORM which is
#  automatically set by the docker engine.

#RUN case ${TARGETPLATFORM} in \
#         "linux/amd64")  GOARCH=amd64  ;; \
#         "linux/arm64")  GOARCH=arm64  ;; \
#         "linux/ppc64le")  GOARCH=ppc64le  ;; \
#         "linux/arm/v6") GOARCH=arm GOARM=6  ;; \
#    esac && \
#    printf "Building OAuth2 Proxy for arch ${GOARCH}\n" && \
#    GOARCH=${GOARCH} VERSION=${VERSION} make build && touch jwt_signing_key.pem

#RUN make build
RUN go build  -o oauth2-k8s-proxy
# Copy binary to alpine
FROM alpine:3.15
COPY nsswitch.conf /etc/nsswitch.conf
#COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/src/github.com/djkormo/oauth2-k8s-proxy /bin/oauth2-k8s-proxy
#COPY --from=builder /go/src/github.com/djkormo/oauth2-k8s-proxy/jwt_signing_key.pem /etc/ssl/private/jwt_signing_key.pem

USER 2000:2000

ENTRYPOINT ["/bin/oauth2-k8s-proxy"]



