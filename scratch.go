package main

import (
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
)

func main() {
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	sess, err := session.Get("cs_test_a1F16P9rjAk2gxzD9JVkGnxDiKM5NSLW9ohOFD6OasvmrVbGBezYgR4cvr", nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Session ID: %s\n", sess.ID)
	fmt.Printf("Payment Status: %s\n", sess.PaymentStatus)
	fmt.Printf("Metadata: %+v\n", sess.Metadata)
}
