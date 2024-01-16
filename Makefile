GOOS=linux

test:
	GOOS=${GOOS} GOARCH=${GOARCH} go test ./...

build:
	mkdir -p bin/extensions
	GOOS=${GOOS} GOARCH=${GOARCH} go build -o bin/extensions/axiom-lambda-extension .

package: build
	cd bin && zip -r extension.zip extensions

publish: package
	aws lambda publish-layer-version --layer-name axiom-development-lambda-extension-go --region eu-west-1 --zip-file "fileb://bin/extension.zip" --compatible-architectures ${GOARCH} --description 'axiom lambda extension to push lambda logs to https://axiom.co' 

arch:
	echo ${GOARCH}

clean:
	rm -r ./bin
