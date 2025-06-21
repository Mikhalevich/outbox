FROM golang:1.24 as builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /app/postgres

FROM scratch
COPY --from=builder /app/postgres /app/postgres

WORKDIR /app/
ENTRYPOINT ["./postgres"]
