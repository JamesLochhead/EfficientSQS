precommit:
	cd src/intermediate_q_process && go fmt . && go mod tidy && golangci-lint run
	cd src/store_sqs_process && go fmt . && go mod tidy && golangci-lint run
insecure-dev-valkey:
	docker build -t insecure-dev-valkey:latest ./dev
	docker run -d -p 6379:6379 --name insecure-dev-valkey insecure-dev-valkey:latest
rm-insecure-dev-valkey:
	docker rm -f insecure-dev-valkey
