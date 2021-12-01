FROM golang:latest

WORKDIR /disysminiproject3

COPY . ./

RUN go mod download

ARG APPTYPE
ARG PORT
ENV APPTYPE ${APPTYPE}
ENV PORT ${PORT}
RUN go build -o ${APPTYPE}/ ${APPTYPE}/${APPTYPE}.go

EXPOSE ${PORT}

CMD ./$APPTYPE/$APPTYPE -port $PORT