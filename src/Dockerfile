FROM golang:1.24 AS build
WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /shopping-cart-api .

FROM gcr.io/distroless/static-debian12
COPY --from=build /shopping-cart-api /shopping-cart-api
EXPOSE 8080
ENTRYPOINT ["/shopping-cart-api"]
