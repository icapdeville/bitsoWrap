FROM golang:1.25-alpine

RUN apk add --no-cache git curl tzdata
ENV TZ=America/Mexico_City

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bitsoWrap ./cmd/server && go build -o bitso-sync ./cmd/sync

ENV PORT=8080
EXPOSE 8080

CMD ["./bitsoWrap"]
