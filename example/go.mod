module main

go 1.21

replace notdiamond => ../

require (
	github.com/joho/godotenv v1.5.1
	notdiamond v0.0.0-00010101000000-000000000000
)

require (
	github.com/mattn/go-sqlite3 v1.14.24 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/schollz/sqlite3dump v1.3.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/sys v0.30.0 // indirect
)
