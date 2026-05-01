package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"

	"sms-ingest/internal/app"
)

func main() {
	h, err := app.NewHandler()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if os.Getenv("LOCAL_HTTP") == "1" {
		if err := app.RunLocalHTTP(h); err != nil {
			fmt.Fprintf(os.Stderr, "local HTTP server stopped: %v\n", err)
			os.Exit(1)
		}
		return
	}
	lambda.Start(h.Handle)
}
