precommit:
	go fmt .
	#go mod tidy
	golangci-lint run
insecure-dev-valkey:
	docker build -t insecure-dev-valkey:latest ./valkey_dev
	docker run -d -p 6379:6379 --name insecure-dev-valkey insecure-dev-valkey:latest
rm-insecure-dev-valkey:
	docker rm -f insecure-dev-valkey
