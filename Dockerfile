FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.24.3-alpine3.21 AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR $GOPATH/src/mypackage/myapp/
COPY ./ .
RUN go get -d -v
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /go/bin/agent_lifecycle_controller


FROM alpine:latest
RUN echo "@testing http://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
 && apk update \
 && apk --no-cache add ca-certificates wakeonlan@testing openssh
WORKDIR /go/bin
COPY --from=builder /go/bin/agent_lifecycle_controller /go/bin/agent_lifecycle_controller
ENTRYPOINT ["/go/bin/agent_lifecycle_controller"]