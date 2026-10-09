FROM node:24-alpine AS dev
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
WORKDIR /app
COPY package.json ./
RUN pnpm --version
EXPOSE 5173
CMD ["sh", "-c", "pnpm install --frozen-lockfile && exec pnpm dev --host 0.0.0.0"]
