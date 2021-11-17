compile-proto: 
	docker run --rm -v "$(PWD)":/proto -w "/proto" rvolosatovs/protoc --go_out=plugins=grpc:/proto --proto_path /proto proto/auction.proto