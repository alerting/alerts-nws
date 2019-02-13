FROM golang:1.11-alpine AS base

# Install dependencies
RUN apk --update --no-cache add git curl

ARG DEP_VERSION=0.5.0

# Download dep binary to bin folder in $GOPATH
RUN mkdir -p /usr/local/bin \
    && curl -fsSL -o /usr/local/bin/dep https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 \
    && chmod +x /usr/local/bin/dep

WORKDIR /go/src/github.com/alerting/alerts-nws
COPY . /go/src/github.com/alerting/alerts-nws

RUN dep ensure
RUN CGO_ENABLED=0 GOOS=linux go install .

FROM alpine:3.9
RUN apk --update --no-cache add ca-certificates
RUN mkdir /polygons \
    && wget -O /polygons/ugc-c.zip https://www.weather.gov/source/gis/Shapefiles/County/c_05mr19.zip \
    && wget -O /polygons/ugc-z.zip https://www.weather.gov/source/gis/Shapefiles/WSOM/z_05mr19.zip
COPY --from=base /go/bin/alerts-nws /alerts-nws
COPY entrypoint.sh /entrypoint.sh
USER 10000
ENTRYPOINT ["/entrypoint.sh"]
