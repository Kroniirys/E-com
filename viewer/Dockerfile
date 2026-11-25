FROM node:24-alpine AS build
WORKDIR /src/viewer
COPY viewer/package.json viewer/package-lock.json ./
RUN npm ci
COPY was/api/openapi.yaml /src/was/api/openapi.yaml
COPY viewer/ ./
RUN npm run build

FROM nginx:1.28-alpine
COPY viewer/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/viewer/dist /usr/share/nginx/html
EXPOSE 80
