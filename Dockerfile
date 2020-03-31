FROM node as novnc-builder

COPY noVNC /noVNC

WORKDIR /noVNC

RUN npm install \
 && ./utils/use_require.js --with-app --as commonjs

FROM golang as go-builder

WORKDIR /app/

COPY go.* ./

RUN go mod download

COPY *.go ./

COPY --from=novnc-builder /noVNC/build/ assets/

RUN mv assets/vnc.html assets/index.html

RUN go generate -x tools.go \
 && go build -v -ldflags '-s -w' -o /gowebsockify

FROM brimstone/debian:sid

RUN package tigervnc-standalone-server locales sudo openbox \
	xterm

RUN locale-gen en_US.UTF-8

ENV LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8 \
    VNC_PASSWD=password \
    PORT=9000

COPY --from=go-builder /gowebsockify /gowebsockify

COPY entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
