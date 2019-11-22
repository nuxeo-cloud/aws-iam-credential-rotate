# Start by building the application.
FROM golang:1.12 as build
ENV GO111MODULE=on
RUN apt-get update -y && apt-get install -y upx
WORKDIR /go/src/aws-iam-credential-rotate
COPY . .
RUN go get -d -v ./...
RUN go install -ldflags="-w -s" -v ./... && \
    upx /go/bin/aws-iam-credential-rotate

# Now copy it into our base image.
FROM gcr.io/distroless/base
COPY --from=build /go/bin/aws-iam-credential-rotate /

ENTRYPOINT [ "/aws-iam-credential-rotate"]
CMD [ "rotate"]