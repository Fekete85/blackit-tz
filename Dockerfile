# A poligon-adat a binárisba fordul, ezért a futtató réteg üres alpine lehet:
# futásidőben semmit nem tölt le és nem hív ki.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
RUN go mod tidy && go build -trimpath -ldflags="-s -w" -o /tz .

FROM alpine:3.21
RUN adduser -D -u 10001 tz
USER tz
COPY --from=build /tz /tz
EXPOSE 8080
ENTRYPOINT ["/tz"]
