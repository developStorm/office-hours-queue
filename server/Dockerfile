FROM golang:1.24 AS builder

LABEL maintainer="Carson Hoffman <hoffcar@umich.edu>"

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ./main .

FROM alpine:3

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
