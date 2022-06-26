FROM golang:latest as builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /go/bin/postgre

FROM scratch
COPY --from=builder /go/bin/postgre /app/postgre

WORKDIR /app/
ENTRYPOINT ["./postgre"]
