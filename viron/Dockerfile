FROM node:9 as viron

RUN git clone -b v1.3.0 https://github.com/pyama86/viron.git /viron

RUN chown -R node:node /viron
ENV HOME /viron

USER node
WORKDIR $HOME
COPY package.json package.json
RUN npm install
RUN sed -i "s|host: 'localhost'|host: '0.0.0.0'|g" /viron/rollup.local.config.js
RUN sed -i "s|ssl: true,|ssl: false,|g" /viron/rollup.local.config.js
RUN sed -i "s|port: 8080|port: 9090|g" /viron/rollup.local.config.js
EXPOSE 9090
USER root
CMD npm start
