# syntax=docker/dockerfile:1
FROM golang:latest as builder
ENV CGO_ENABLED 0
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . ./
RUN rm -rf build
RUN go build -o /waitingroom

FROM scratch
COPY --from=builder /waitingroom /waitingroom
EXPOSE 18080
CMD [ "/waitingroom", "server"]
