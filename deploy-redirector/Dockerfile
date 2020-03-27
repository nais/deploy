FROM node:13-alpine

EXPOSE 8080
WORKDIR /

COPY ./package.json ./
COPY ./index.js ./
RUN npm install

CMD ["node", "./index.js"]