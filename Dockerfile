# All builds should be done using the platform native to the build node to allow
#  cache sharing of the go mod download step.

FROM  golang:1.18-buster AS builder

# Copy sources
WORKDIR $GOPATH/src/github.com/djkormo/oauth2-k8s-proxy

# Fetch dependencies
COPY go.mod go.sum ./

RUN go mod download

# Now pull in our code
COPY . .

RUN go build  -o oauth2-k8s-proxy
# Copy binary to alpine
FROM alpine:3.15
COPY nsswitch.conf /etc/nsswitch.conf

COPY --from=builder /go/src/github.com/djkormo/oauth2-k8s-proxy /bin/oauth2-k8s-proxy

USER 2000:2000

ENTRYPOINT ["/bin/oauth2-k8s-proxy"]



