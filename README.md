# disysminiproject3

## Starting the service

Requirements:
- docker
- docker-compose

To start all of the services, use: 

docker-compose up

if you're developing and want to test new code remember to add --build to rebuild the images.

docker-compose up --builds

If you want to define an additional service, you can add it in the docker-compose.yml file


## Requesting the API

To get the result use GET at 8000/result

To make a bid use POST at 8000/bid

with the post form "userid" and "amount"