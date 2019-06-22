FROM arm32v7/golang:1.12.6-alpine

COPY . /app
WORKDIR /app

RUN go build -mod=vendor -o /godns .
