FROM alpine:3
RUN apk add --no-cache curl vim lsof ca-certificates

# RUN `CGO_ENABLED=0 GOOS=linux go build -o hello-server server/main.go` frirst
COPY ./hello-server /hello-server

# Run the web service on container startup.
CMD ["/hello-server"]
