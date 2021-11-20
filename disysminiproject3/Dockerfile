FROM golang:latest

WORKDIR /disysminiproject3

COPY . ./

RUN go mod download

ARG APPTYPE

RUN go build -o ${APPTYPE}/ ${APPTYPE}/${APPTYPE}.go

EXPOSE 8080

CMD [ "./${APPTYPE}/${APPTYPE}" ]