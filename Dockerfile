FROM golang:1.22.3

WORKDIR /locksmith

COPY go.mod go.sum .

RUN go mod download

COPY . .

RUN go install -v ./...

RUN GOOS=windows go install -v ./...
RUN GOOS=darwin go install -v ./...
