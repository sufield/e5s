FROM golang:1.23 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/client ./examples/container-client

FROM gcr.io/distroless/base-debian12
WORKDIR /work
COPY --from=build /out/client /bin/client
COPY testdata/e5s-client.yaml /work/e5s-client.yaml
ENTRYPOINT ["/bin/client"]
