# disysminiproject3

Requirements:
- docker
- docker-compose

To start all of the services, use: 

docker-compose up

if you're developing and want to test new code remember to add --build to rebuild the images.

docker-compose up --builds

If you want to define an additional service, you can add it in the docker-compose.yml file

Clients dont need to have ports exposed, frontends and servers do. 

