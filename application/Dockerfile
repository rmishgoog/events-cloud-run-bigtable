#Using a multi-staged build, build the go binary here
FROM golang:1.18 as build

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/climate-updates

#Now copy the binary into our base image (distroless debian)
FROM gcr.io/distroless/static-debian11
COPY --from=build /go/bin/climate-updates /
CMD ["/climate-updates"]