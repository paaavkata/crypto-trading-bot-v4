module github.com/paaavkata/crypto-trading-bot-v4/trading-engine

go 1.23.3

require (
	github.com/google/uuid v1.4.0
	github.com/markcheno/go-talib v0.0.0-20250114000313-ec55a20c902f
	github.com/paaavkata/crypto-trading-bot-v4/shared v0.0.0-20250528155433-b5b9ac4e36cc
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.7.0
)

replace github.com/paaavkata/crypto-trading-bot-v4/shared => ../../shared

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-resty/resty/v2 v2.16.5 // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)
