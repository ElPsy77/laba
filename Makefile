# Counterpart of the Maven Wrapper: ./mvnw spring-boot:run, ./mvnw clean package
APP := target/spring-lab-01

.PHONY: run build clean test

run:            ## ./mvnw spring-boot:run
	go run ./cmd/springlab $(ARGS)

build: clean    ## ./mvnw clean package
	go build -o $(APP) ./cmd/springlab

test:           ## ./mvnw test
	go test ./...

clean:
	rm -rf target
