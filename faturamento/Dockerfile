# Estágio 1: compila. Usa a imagem do Go, que traz o compilador inteiro (~800MB).
FROM golang:1.26-alpine AS build

WORKDIR /src

# go.mod e go.sum copiados antes do resto do código de propósito: enquanto eles não
# mudarem, o Docker reaproveita a camada do `go mod download` em vez de baixar tudo
# de novo a cada build. Sem isso, mexer numa linha de código refaz o download inteiro.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 gera um binário estático, sem depender de bibliotecas C do sistema.
# É o que permite rodar numa imagem mínima no estágio seguinte.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/faturamento ./cmd/api

# Estágio 2: só o binário. O compilador fica para trás.
FROM alpine:3.21

RUN adduser -D -u 10001 app
USER app

COPY --from=build /bin/faturamento /bin/faturamento

EXPOSE 7081
ENTRYPOINT ["/bin/faturamento"]
