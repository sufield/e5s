FROM golang:1.23 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./examples/container-server

FROM gcr.io/distroless/base-debian12
WORKDIR /work
COPY --from=build /out/server /bin/server
COPY testdata/e5s-server.yaml /work/e5s-server.yaml
# socket volume will mount at /spire-agent
ENTRYPOINT ["/bin/server"]
