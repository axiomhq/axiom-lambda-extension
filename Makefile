GOOS=linux
GOARCH=amd64

build:
	mkdir -p bin/extensions
	GOOS=${GOOS} GOARCH=${GOARCH} go build -o bin/extensions/axiom-lambda-extension .

publish: build
	cd bin && zip -r extension.zip extensions 

