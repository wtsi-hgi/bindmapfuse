FROM ubuntu:16.04

MAINTAINER jcrandall@alum.mit.edu

RUN \
  apt-get -qqy update && \
  apt-get -qqy install --no-install-recommends \
    bash \
    build-essential \
    ca-certificates \
    curl \
    gcc \
    git \
    libfuse-dev && \
  rm -rf /var/lib/apt/lists/* && \
  export TMPDIR=$(mktemp -d) && \
  cd ${TMPDIR} && \
  curl -fsSL https://dl.google.com/go/go1.11.linux-amd64.tar.gz -o golang.tar.gz && \
  echo "b3fcf280ff86558e0559e185b601c9eade0fd24c900b4c63cd14d1d38613e499  golang.tar.gz" | sha256sum -c - || (echo "Failed to verify download hash"; exit 1) && \
  tar -C /usr/local -xzf golang.tar.gz && \
  cd && \
  rm -rf "${TMPDIR}"

ENV GOPATH=/go
ENV PATH=${GOPATH}/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN mkdir -p ${GOPATH}/src/github.com/wtsi-hgi/bindmapfuse
ADD *.go ${GOPATH}/src/github.com/wtsi-hgi/bindmapfuse/
WORKDIR ${GOPATH}/src/github.com/wtsi-hgi/bindmapfuse
RUN \
  go get && \
  go build && \
  go install
