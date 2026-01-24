generate_sql:
	rm -rf contoller/dao
	sqlc generate

start_server: generate_sql
	go run main.go server

format: generate_sql
	gofumpt -l -w .

build-binary: format
	mkdir -p "build"
	GOOS=windows GOARCH=amd64 go build -o build/vestigo.exe main.go
	ls -alh build/

test: format
	go test -v ./...
