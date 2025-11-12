FROM golang:1.23@sha256:60deed95d3888cc5e4d9ff8a10c54e5edc008c6ae3fba6187be6fb592e19e8c0 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/client ./examples/container-client

FROM gcr.io/distroless/base-debian12@sha256:9e9b50d2048db3741f86a48d939b4e4cc775f5889b3496439343301ff54cdba8
WORKDIR /work
COPY --from=build /out/client /bin/client
COPY testdata/e5s-client.yaml /work/e5s-client.yaml
ENTRYPOINT ["/bin/client"]
