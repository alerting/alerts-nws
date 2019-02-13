SRCS = $(shell git ls-files '*.go' | grep -v '^vendor/')

build: $(SRCS)
	docker build -t nexus-docker.zacharyseguin.ca/alerts/alerts-nws:latest .

push: build
	docker push nexus-docker.zacharyseguin.ca/alerts/alerts-nws:latest
