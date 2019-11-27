FROM golang:1.12.6-alpine AS builder

RUN apk update && apk upgrade && \
    apk add --no-cache bash git openssh ca-certificates

COPY . /app
WORKDIR /app

ENV GO111MODULE=on
RUN go mod download

RUN CGO_ENABLED=0 go build -o /godns .

FROM alpine

COPY --from=builder /godns /godns
ENTRYPOINT ["/godns"]
