generate_sql:
	rm -rf contoller/dao
	sqlc generate

start_server: generate_sql
	go run main.go server

format: generate_sql
	gofumpt -l -w .

build-binary: format
	mkdir -p "build"
	go build -o build/vestigo main.go
	ls -alh build/

test: format
	go test -v ./...
