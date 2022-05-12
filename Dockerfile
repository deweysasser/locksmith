FROM golang:1.18-bullseye

WORKDIR /go/src/github.com/deweysasser/locksmith

RUN go get -d -v \
    github.com/aws/aws-sdk-go      \
    github.com/urfave/cli          \
    github.com/deweysasser/pkcs8   \
    github.com/remeh/sizedwaitgroup 
    
COPY . .
RUN go get -d -v ./...
RUN go install -v ./...

RUN GOOS=windows go install -v ./...
RUN GOOS=darwin go install -v ./...