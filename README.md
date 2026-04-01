the monolithic arch is inside /internal
can run it using a single binary that is cmd/server/main.go

other directories inside cmd are microservices made by spliting monolithic using same logic
but with added gRPC calls between services, and an actual postgresSQL server

to run distributed arch, compose the docker file or use 4 terminals to run all 4 servies
