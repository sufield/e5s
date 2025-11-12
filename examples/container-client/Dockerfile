FROM golang:1.25.3@sha256:6d4e5e74f47db00f7f24da5f53c1b4198ae46862a47395e30477365458347bf2 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/client ./examples/container-client

FROM gcr.io/distroless/base-debian12@sha256:9e9b50d2048db3741f86a48d939b4e4cc775f5889b3496439343301ff54cdba8
WORKDIR /work
COPY --from=build /out/client /bin/client
COPY testdata/e5s-client.yaml /work/e5s-client.yaml
ENTRYPOINT ["/bin/client"]
