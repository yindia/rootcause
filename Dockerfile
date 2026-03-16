FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/rootcause .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/rootcause /usr/local/bin/rootcause

EXPOSE 8000

ENTRYPOINT ["/usr/local/bin/rootcause"]
CMD ["--transport", "stdio"]
