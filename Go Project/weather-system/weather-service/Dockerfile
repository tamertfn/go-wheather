FROM golang:1.23.4-alpine

# Go modül yapısına uygun workspace
WORKDIR /go/src/weather-system/weather-service

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /go/bin/app

EXPOSE 8080

CMD ["/go/bin/app"]
