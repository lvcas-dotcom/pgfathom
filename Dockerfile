# The binary arrives ready: goreleaser has already compiled it without cgo and
# stamped the version. Compiling again here would produce an artifact different
# from the one that was published and tested, which is the easiest way for an
# image to lie about what it contains.
#
# The context is a temporary directory goreleaser assembles with one binary per
# platform, in subdirectories — linux/amd64/pgfathom, linux/arm64/pgfathom.
# That is where TARGETPLATFORM comes from: copying from the root of the context
# worked under the old configuration format and fails under this one, with
# "/pgfathom: not found" during the multi-platform build.
#
# distroless/static rather than scratch. The tool opens a TLS connection to the
# user's server, and the configuration recommended for production verifies the
# certificate. With no certificate authority inside the image that fails with a
# message about an unknown certificate that reads like a fault in their server —
# and the first run of whoever chose the container becomes a bug report. The
# nonroot variant runs as an unprivileged user, which is what a read-only tool
# needs and nothing more.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM

COPY ${TARGETPLATFORM}/pgfathom /usr/local/bin/pgfathom

ENTRYPOINT ["/usr/local/bin/pgfathom"]
