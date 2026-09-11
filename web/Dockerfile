# The frontend is static files. Build them with Node, serve them with nginx,
# and ship neither Node nor the sources.
#
# The build stage runs on the builder's own architecture: its output is the
# same bytes either way, and only the nginx that serves them is per-platform.
FROM --platform=$BUILDPLATFORM node:22-alpine AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
