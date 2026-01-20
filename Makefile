generate_sql:
	rm -rf contoller/dao
	sqlc generate

start_server: generate_sql
	go run main.go server

build-binary: generate_sql
	mkdir -p "build"
	go build -o build/vestigo main.go

format:
	gofumpt -l -w .