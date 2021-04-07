FROM docker.io/openshift/origin-release:golang-1.14 AS builder
WORKDIR /go/src/github.com/open-cluster-management/failover
COPY . .
ENV GO_PACKAGE github.com/open-cluster-management/failover

RUN make build --warn-undefined-variables
RUN make build-e2e --warn-undefined-variables

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest
COPY --from=builder /go/src/github.com/open-cluster-management/failover/failover /
COPY --from=builder /go/src/github.com/open-cluster-management/failover/e2e.test /
RUN microdnf update && microdnf clean all
