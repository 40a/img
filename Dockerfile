ARG RUNC_VERSION=9f9c96235cc97674e935002fc3d78361b696a69e
FROM golang:1.9-alpine AS gobuild-base
RUN apk add --no-cache git make

FROM gobuild-base AS runc
ARG RUNC_VERSION
RUN apk add --no-cache \
	bash \
	curl \
	g++ \
	libseccomp-dev \
	linux-headers
RUN git clone https://github.com/AkihiroSuda/runc.git "$GOPATH/src/github.com/opencontainers/runc" \
	&& cd "$GOPATH/src/github.com/opencontainers/runc" \
	&& git checkout -q "unshare-m-r" \
	&& make static BUILDTAGS="seccomp" EXTRA_FLAGS="-buildmode pie" EXTRA_LDFLAGS="-extldflags \\\"-fno-PIC -static\\\"" \
	&& mv runc /usr/bin/runc

FROM gobuild-base AS img
WORKDIR /go/src/github.com/jessfraz/img
COPY . .
RUN make static && mv img /usr/bin/img

FROM alpine
MAINTAINER Jessica Frazelle <jess@linux.com>
RUN apk add --no-cache \
	fuse \
	git \
	shadow-uidmap
COPY --from=img /usr/bin/img /usr/bin/img
COPY --from=runc /usr/bin/runc /usr/bin/runc
ENTRYPOINT [ "img" ]
CMD [ "--help" ]
