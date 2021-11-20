compile-proto: 
	docker run --rm -v "$(PWD)":/proto -w "/proto" rvolosatovs/protoc --go_out=plugins=grpc:/proto --proto_path /proto proto/auction.proto

build-server-image:
	docker build --build-arg APPTYPE=server .

build-frontend-image:
	docker build --build-arg APPTYPE=frontend .

build-client-image:
	docker build --build-arg APPTYPE=client .