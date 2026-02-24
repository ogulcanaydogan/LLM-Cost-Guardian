FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/lcg ./cmd/lcg
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/guardian ./cmd/guardian

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/lcg /usr/local/bin/lcg
COPY --from=builder /bin/guardian /usr/local/bin/guardian
COPY pricing/ /etc/lcg/pricing/

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["guardian"]
CMD ["--listen", ":8080"]
