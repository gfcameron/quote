# Build Stage
# First pull Golang image
FROM golang:1.19-alpine as build
 
# Set environment variable
ENV APP_NAME quote
ENV CMD_PATH quote.go

WORKDIR $GOPATH/src

# Copy application data into image
COPY ./src .

# Install tools required for project
# Run `docker build --no-cache .` to update dependencies
RUN apk add --no-cache git
RUN go get -u "github.com/justinas/alice"

# Install any library dependencies (none as of yet)
# RUN dep ensure -vendor-only

# Build application
RUN CGO_ENABLED=0 go build -v -o /${APP_NAME} $GOPATH/src/$CMD_PATH

# Run stage
FROM alpine:3.17

WORKDIR /bin

# Copy only required data into this image
COPY --from=build /quote .

# Expose application port
EXPOSE 8443

ENTRYPOINT ["/bin/quote"]



