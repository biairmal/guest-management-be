module github.com/biairmal/guest-management-be/mocks

go 1.25.1

replace github.com/biairmal/guest-management-be => ../

replace github.com/biairmal/go-sdk => ../../go-sdk

require (
	github.com/biairmal/go-sdk v0.0.1
	github.com/biairmal/guest-management-be v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	go.uber.org/mock v0.6.0
)

require (
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-chi/chi/v5 v5.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.3 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
