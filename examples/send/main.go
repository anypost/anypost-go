// Command send sends a single email.
//
// Run with an API key in the environment:
//
//	ANYPOST_API_KEY=ap_... go run ./examples/send
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/anypost/anypost-go"
)

func main() {
	// Reads ANYPOST_API_KEY from the environment. Pass the key explicitly with
	// anypost.New("ap_...") if you prefer.
	client, err := anypost.New("")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sent, err := client.Email.Send(context.Background(), &anypost.SendEmailRequest{
		From:    "Acme <you@yourdomain.com>",
		To:      []string{"someone@example.com"},
		Subject: "Hello from Anypost",
		HTML:    "<p>It worked.</p>",
	})
	if err != nil {
		var apiErr *anypost.Error
		if errors.As(err, &apiErr) && apiErr.Type == anypost.ErrorTypeValidation {
			fmt.Fprintf(os.Stderr, "validation failed: %v\n", apiErr.ValidationErrors)
		} else {
			fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Queued %s\n", sent.ID)
}
