# Estágio 1: compila o Angular. Precisa do Node e das dependências de build.
FROM node:24-alpine AS build

WORKDIR /src

# package.json antes do resto do código: enquanto as dependências não mudarem, o
# Docker reaproveita a camada do npm ci em vez de reinstalar tudo a cada build.
COPY package*.json ./
RUN npm ci

COPY . .
RUN npm run build

# Estágio 2: só os arquivos estáticos, servidos pelo nginx.
# O Node fica para trás — a aplicação compilada é HTML, CSS e JS.
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist/web/browser /usr/share/nginx/html

EXPOSE 80
